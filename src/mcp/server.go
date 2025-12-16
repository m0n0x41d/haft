package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// JSON-RPC Types
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

// MCP Protocol Types
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

// Server handles the MCP loop
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
			// Ignore unknown notifications, error on requests
			if req.ID != nil {
				s.sendError(req.ID, -32601, "Method not found")
			}
		}
	}
}

func (s *Server) send(resp JSONRPCResponse) {
	bytes, _ := json.Marshal(resp)
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

// --- Handlers ---

func (s *Server) handleInitialize(req JSONRPCRequest) {
	s.sendResult(req.ID, map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]string{
			"name":    "quint-mcp",
			"version": "0.1.0",
		},
	})
}

func (s *Server) handleToolsList(req JSONRPCRequest) {
	tools := []Tool{
		{
			Name:        "fpf_status",
			Description: "Get current FPF phase and context.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "fpf_init",
			Description: "Initialize FPF project structure.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"role": map[string]string{"type": "string", "description": "Acting role (Abductor)"},
				},
				"required": []string{"role"},
			},
		},
		{
			Name:        "fpf_context",
			Description: "Get agent system prompt for a role.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"role": map[string]string{"type": "string", "description": "Abductor, Deductor, Inductor, etc."},
				},
				"required": []string{"role"},
			},
		},
		{
			Name:        "fpf_propose",
			Description: "Propose a new hypothesis (L0).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"role":    map[string]string{"type": "string"},
					"title":   map[string]string{"type": "string"},
					"content": map[string]string{"type": "string"},
				},
				"required": []string{"role", "title", "content"},
			},
		},
		{
			Name:        "fpf_evidence",
			Description: "Add evidence or logic checks to a hypothesis.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"role":      map[string]string{"type": "string"},
					"action":    map[string]interface{}{"type": "string", "enum": []interface{}{"add", "check"}},
					"target_id": map[string]string{"type": "string"},
					"type":      map[string]interface{}{"type": "string", "enum": []interface{}{"internal", "external", "logic"}},
					"content":   map[string]string{"type": "string"},
					"verdict":   map[string]interface{}{"type": "string", "enum": []interface{}{"PASS", "FAIL", "REFINE"}},
				},
				"required": []string{"role", "target_id", "content", "verdict"},
			},
		},
		{
			Name:        "fpf_loopback",
			Description: "Trigger Loopback (Induction -> Deduction) on failure.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"role":      map[string]string{"type": "string"},
					"parent_id": map[string]string{"type": "string"},
					"insight":   map[string]string{"type": "string"},
					"new_title": map[string]string{"type": "string"},
					"new_content": map[string]string{"type": "string"},
				},
				"required": []string{"role", "parent_id", "insight", "new_title", "new_content"},
			},
		},
		{
			Name:        "fpf_decide",
			Description: "Finalize decision (DRR).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"role":      map[string]string{"type": "string"},
					"title":     map[string]string{"type": "string"},
					"winner_id": map[string]string{"type": "string"},
					"content":   map[string]string{"type": "string"},
				},
				"required": []string{"role", "title", "winner_id", "content"},
			},
		},
		{
			Name:        "fpf_transition",
			Description: "Explicitly request phase transition.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"role":   map[string]string{"type": "string"},
					"target": map[string]string{"type": "string"},
				},
				"required": []string{"role", "target"},
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

	// Helper to extract string arg
	arg := func(k string) string {
		if v, ok := params.Arguments[k].(string); ok {
			return v
		}
		return ""
	}

	var output string
	var err error

	// Wrap FSM checks inside the tools calls implicitly via shared logic,
	// or call them here. Since Tools struct handles logic, we delegate.
	
	// NOTE: We must instantiate the FSM check here or in Tools. 
	// The Tools struct we built earlier checks FSM. 
	// We just need to map args.

	switch params.Name {
	case "fpf_status":
		st := s.tools.FSM.State.Phase
		output = string(st)

	case "fpf_init":
		res := s.tools.InitProject()
		if res != nil {
			err = res
		} else {
			s.tools.FSM.State.Phase = PhaseAbduction
			saveErr := s.tools.FSM.SaveState(s.tools.getFPFDir() + "/state.json")
			if saveErr != nil {
				err = fmt.Errorf("failed to save FSM state after init: %v", saveErr)
			} else {
				output = "Initialized. Phase: ABDUCTION"
			}
		}

	case "fpf_context":
		role := arg("role")
		output, err = s.tools.GetAgentContext(role)

	case "fpf_propose":
		role := arg("role")
		ok, msg := s.tools.FSM.CanTransition(PhaseAbduction, Role(role))
		if !ok {
			err = fmt.Errorf("%s", msg)
		} else {
			output, err = s.tools.ProposeHypothesis(arg("title"), arg("content"))
		}

	case "fpf_evidence":
		role := arg("role")
		// Check if role valid for CURRENT phase
		if isValidRoleForPhase(s.tools.FSM.State.Phase, Role(role)) {
			output, err = s.tools.ManageEvidence(s.tools.FSM.State.Phase, arg("target_id"), arg("type"), arg("content"), arg("verdict"))
		} else {
			err = fmt.Errorf("Role %s cannot act in %s", role, s.tools.FSM.State.Phase)
		}

	case "fpf_loopback":
		role := arg("role")
		ok, msg := s.tools.FSM.CanTransition(PhaseDeduction, Role(role))
		if !ok {
			err = fmt.Errorf("%s", msg)
		} else {
			output, err = s.tools.RefineLoopback(s.tools.FSM.State.Phase, arg("parent_id"), arg("insight"), arg("new_title"), arg("new_content"))
			if err == nil {
				s.tools.FSM.State.Phase = PhaseDeduction
				if saveErr := s.tools.FSM.SaveState(s.tools.getFPFDir() + "/state.json"); saveErr != nil {
					err = fmt.Errorf("failed to save FSM state after loopback: %v", saveErr)
				}
			}
		}

	case "fpf_decide":
		role := arg("role")
		// Ensure in Decision phase
		if s.tools.FSM.State.Phase == PhaseInduction {
			// Auto-transition if valid
			ok, msg := s.tools.FSM.CanTransition(PhaseDecision, Role(role))
					if !ok {
						err = fmt.Errorf("%s", msg)
					} else {				s.tools.FSM.State.Phase = PhaseDecision
			}
		}
		
		if err == nil {
			output, err = s.tools.FinalizeDecision(arg("title"), arg("content"), arg("winner_id"))
			if err == nil {
				s.tools.FSM.State.Phase = PhaseIdle
				if saveErr := s.tools.FSM.SaveState(s.tools.getFPFDir() + "/state.json"); saveErr != nil {
					err = fmt.Errorf("failed to save FSM state after decision: %v", saveErr)
				}
			}
		}

	case "fpf_transition":
		role := arg("role")
		target := Phase(arg("target"))
		ok, msg := s.tools.FSM.CanTransition(target, Role(role))
		if !ok {
			err = fmt.Errorf("%s", msg)
		} else {
							s.tools.FSM.State.Phase = target
							if saveErr := s.tools.FSM.SaveState(s.tools.getFPFDir() + "/state.json"); saveErr != nil {
								err = fmt.Errorf("failed to save FSM state after transition: %v", saveErr)
							} else {
								output = fmt.Sprintf("Transitioned to %s", target)
							}		}

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