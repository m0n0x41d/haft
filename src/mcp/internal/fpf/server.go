package fpf

import (
	"bufio"
	"context"
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

// V5ToolHandler handles a v5 MCP tool call and returns the result text.
type V5ToolHandler func(ctx context.Context, toolName string, params json.RawMessage) (string, error)

type Server struct {
	tools     *Tools
	v5Handler V5ToolHandler
}

func NewServer(t *Tools) *Server {
	return &Server{tools: t}
}

// SetV5Handler registers the handler for v5 tools (quint_note, quint_problem, etc).
func (s *Server) SetV5Handler(h V5ToolHandler) {
	s.v5Handler = h
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
			"version": "5.0.0",
		},
	})
}

func (s *Server) handleToolsList(req JSONRPCRequest) {
	var tools []Tool

	// v5 tools only
	if s.v5Handler != nil {
		tools = append(tools,
			Tool{
				Name:        "quint_note",
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
						"context": map[string]string{
							"type":        "string",
							"description": "Optional context name for grouping (e.g., 'auth', 'payments')",
						},
					},
					"required": []string{"title", "rationale"},
				},
			},
			Tool{
				Name:        "quint_problem",
				Description: "Frame, characterize, and manage engineering problems. Actions: 'frame' creates a ProblemCard, 'characterize' adds comparison dimensions, 'select' lists active problems. Frame the problem BEFORE exploring solutions.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type":        "string",
							"enum":        []interface{}{"frame", "characterize", "select"},
							"description": "frame=create ProblemCard, characterize=add comparison dimensions, select=list/filter active problems",
						},
						"title": map[string]string{
							"type":        "string",
							"description": "(frame) Problem title",
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
									"how_to_measure": map[string]string{"type": "string", "description": "How this dimension is measured"},
								},
								"required": []string{"name"},
							},
							"description": "(characterize) Comparison dimensions for evaluating solutions",
						},
						"parity_rules": map[string]string{
							"type":        "string",
							"description": "(characterize) What must be equal across all variants for fair comparison",
						},
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
			Name:        "quint_solution",
			Description: "Explore solution variants and compare them fairly. Actions: 'explore' creates a SolutionPortfolio with >=2 variants (each with weakest link), 'compare' runs parity check and identifies the Pareto front.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []interface{}{"explore", "compare"},
						"description": "explore=create variants portfolio, compare=run parity comparison",
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
								"title":          map[string]string{"type": "string", "description": "Variant name"},
								"description":    map[string]string{"type": "string", "description": "What this option does"},
								"strengths":      map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
								"weakest_link":   map[string]string{"type": "string", "description": "What bounds this option's quality (WLNK)"},
								"risks":          map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
								"stepping_stone": map[string]interface{}{"type": "boolean", "description": "Opens future possibilities even if not optimal now"},
								"rollback_notes": map[string]string{"type": "string"},
							},
							"required": []string{"title", "weakest_link"},
						},
						"description": "(explore) Solution variants — at least 2, genuinely distinct",
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
						"description": "(compare) Variant IDs on the Pareto front",
					},
					"policy_applied": map[string]string{
						"type":        "string",
						"description": "(compare) Selection policy that was applied",
					},
					"selected_ref": map[string]string{
						"type":        "string",
						"description": "(compare) Recommended variant ID",
					},
					"context": map[string]string{
						"type":        "string",
						"description": "Optional context name",
					},
					"mode": map[string]string{
						"type":        "string",
						"description": "(explore) Decision mode: tactical, standard (default), deep",
					},
				},
				"required": []string{"action"},
			},
		})
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
			Content: []ContentItem{{Type: "text", Text: "Quint Code not initialized. Run: quint-code init"}},
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
