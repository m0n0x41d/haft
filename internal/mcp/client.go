// Package mcp provides a client for connecting to external MCP servers.
// MCP (Model Context Protocol) servers expose tools and resources over stdio.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// ServerConfig describes how to connect to an MCP server.
type ServerConfig struct {
	Name    string            `json:"name" yaml:"name"`
	Command string            `json:"command" yaml:"command"`
	Args    []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
}

// ToolDef is a tool exposed by an MCP server.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	ServerName  string         `json:"-"` // which server provides this tool
}

// Resource is a resource exposed by an MCP server.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Client manages connections to MCP servers.
type Client struct {
	mu      sync.RWMutex
	servers map[string]*serverConn
}

type serverConn struct {
	config  ServerConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	tools   []ToolDef
	nextID  int
	mu      sync.Mutex
	pending map[int]chan json.RawMessage
}

// NewClient creates an MCP client.
func NewClient() *Client {
	return &Client{
		servers: make(map[string]*serverConn),
	}
}

// Connect starts an MCP server process and discovers its tools.
func (c *Client) Connect(ctx context.Context, cfg ServerConfig) error {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Stderr = os.Stderr

	// Set env
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start MCP server %s: %w", cfg.Name, err)
	}

	conn := &serverConn{
		config:  cfg,
		cmd:     cmd,
		stdin:   stdin,
		scanner: bufio.NewScanner(stdout),
		pending: make(map[int]chan json.RawMessage),
	}

	// Start read loop
	go conn.readLoop()

	// Initialize
	if err := conn.initialize(ctx); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("initialize MCP server %s: %w", cfg.Name, err)
	}

	// Discover tools
	tools, err := conn.listTools(ctx)
	if err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("list tools from %s: %w", cfg.Name, err)
	}

	for i := range tools {
		tools[i].ServerName = cfg.Name
	}
	conn.tools = tools

	c.mu.Lock()
	c.servers[cfg.Name] = conn
	c.mu.Unlock()

	return nil
}

// Tools returns all tools from all connected servers.
func (c *Client) Tools() []ToolDef {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var all []ToolDef
	for _, conn := range c.servers {
		all = append(all, conn.tools...)
	}
	return all
}

// CallTool invokes a tool on the appropriate server.
func (c *Client) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (string, error) {
	c.mu.RLock()
	conn, ok := c.servers[serverName]
	c.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("MCP server not connected: %s", serverName)
	}

	return conn.callTool(ctx, toolName, args)
}

// ListResources returns resources from a server.
func (c *Client) ListResources(ctx context.Context, serverName string) ([]Resource, error) {
	c.mu.RLock()
	conn, ok := c.servers[serverName]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("MCP server not connected: %s", serverName)
	}

	return conn.listResources(ctx)
}

// ReadResource reads a resource from a server.
func (c *Client) ReadResource(ctx context.Context, serverName, uri string) (string, error) {
	c.mu.RLock()
	conn, ok := c.servers[serverName]
	c.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("MCP server not connected: %s", serverName)
	}

	return conn.readResource(ctx, uri)
}

// Close shuts down all server connections.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, conn := range c.servers {
		_ = conn.stdin.Close()
		_ = conn.cmd.Process.Kill()
	}
	c.servers = make(map[string]*serverConn)
}

// --- Server connection internals ---

type jsonrpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *serverConn) send(msg jsonrpcMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = fmt.Fprintf(s.stdin, "%s\n", data)
	return err
}

func (s *serverConn) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	ch := make(chan json.RawMessage, 1)
	s.pending[id] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()

	paramsData, _ := json.Marshal(params)

	if err := s.send(jsonrpcMsg{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  paramsData,
	}); err != nil {
		return nil, err
	}

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("MCP request timeout: %s", method)
	}
}

func (s *serverConn) readLoop() {
	for s.scanner.Scan() {
		line := s.scanner.Bytes()
		var msg jsonrpcMsg
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		if msg.ID != nil {
			s.mu.Lock()
			ch, ok := s.pending[*msg.ID]
			s.mu.Unlock()
			if ok {
				if msg.Error != nil {
					ch <- nil // error case handled by caller
				} else {
					ch <- msg.Result
				}
			}
		}
	}
}

func (s *serverConn) initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "haft",
			"version": "0.1.0",
		},
	}
	_, err := s.request(ctx, "initialize", params)
	if err != nil {
		return err
	}
	// Send initialized notification
	return s.send(jsonrpcMsg{JSONRPC: "2.0", Method: "notifications/initialized"})
}

func (s *serverConn) listTools(ctx context.Context) ([]ToolDef, error) {
	raw, err := s.request(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var result struct {
		Tools []ToolDef `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse tools/list: %w", err)
	}
	return result.Tools, nil
}

func (s *serverConn) callTool(ctx context.Context, name string, args map[string]any) (string, error) {
	params := map[string]any{
		"name":      name,
		"arguments": args,
	}
	raw, err := s.request(ctx, "tools/call", params)
	if err != nil {
		return "", err
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse tools/call result: %w", err)
	}
	var texts []string
	for _, c := range result.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}
	return fmt.Sprintf("%s", joinStrings(texts)), nil
}

func (s *serverConn) listResources(ctx context.Context) ([]Resource, error) {
	raw, err := s.request(ctx, "resources/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var result struct {
		Resources []Resource `json:"resources"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse resources/list: %w", err)
	}
	return result.Resources, nil
}

func (s *serverConn) readResource(ctx context.Context, uri string) (string, error) {
	raw, err := s.request(ctx, "resources/read", map[string]any{"uri": uri})
	if err != nil {
		return "", err
	}
	var result struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse resources/read: %w", err)
	}
	var texts []string
	for _, c := range result.Contents {
		texts = append(texts, c.Text)
	}
	return joinStrings(texts), nil
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += "\n"
		}
		result += s
	}
	return result
}
