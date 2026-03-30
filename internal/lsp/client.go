package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/m0n0x41d/haft/logger"
)

// ---------------------------------------------------------------------------
// L2/L3: LSP Client — JSON-RPC 2.0 over stdio
// ---------------------------------------------------------------------------

// Client manages a single LSP server process and provides protocol operations.
type Client struct {
	config  ServerConfig
	rootURI string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	state  ServerState
	stateMu sync.RWMutex

	nextID atomic.Int64

	// Diagnostics storage: file URI → diagnostics
	diagnostics   map[string][]Diagnostic
	diagnosticsMu sync.RWMutex

	// Open file tracking: URI → version
	openFiles   map[string]int
	openFilesMu sync.Mutex

	// Response channels: request ID → channel
	pending   map[int]chan json.RawMessage
	pendingMu sync.Mutex

	// Callback when diagnostics change
	onDiagnostics func(serverName string, counts DiagnosticCounts)

	done chan struct{} // closed when reader goroutine exits
}

// NewClient creates an LSP client for the given server config.
// Does not start the server — call Start() for that.
func NewClient(cfg ServerConfig, projectRoot string) *Client {
	return &Client{
		config:      cfg,
		rootURI:     FileToURI(projectRoot),
		state:       StateUnstarted,
		diagnostics: make(map[string][]Diagnostic),
		openFiles:   make(map[string]int),
		pending:     make(map[int]chan json.RawMessage),
		done:        make(chan struct{}),
	}
}

// State returns the current server state.
func (c *Client) State() ServerState {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state
}

func (c *Client) setState(s ServerState) {
	c.stateMu.Lock()
	c.state = s
	c.stateMu.Unlock()
}

// SetDiagnosticsCallback sets a function called when diagnostics change.
func (c *Client) SetDiagnosticsCallback(fn func(string, DiagnosticCounts)) {
	c.onDiagnostics = fn
}

// Start launches the LSP server process and initializes the protocol.
func (c *Client) Start(ctx context.Context) error {
	c.setState(StateStarting)

	// Build command with environment
	cmd := exec.CommandContext(ctx, c.config.Command, c.config.Args...)
	cmd.Env = os.Environ()
	for k, v := range c.config.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		c.setState(StateError)
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.setState(StateError)
		return fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard // suppress server stderr

	if err := cmd.Start(); err != nil {
		c.setState(StateError)
		return fmt.Errorf("start %s: %w", c.config.Command, err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = bufio.NewReaderSize(stdout, 64*1024)

	// Start reading responses/notifications in background
	go c.readLoop()

	// Send initialize request
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	params := initializeParams{
		ProcessID: os.Getpid(),
		RootURI:   c.rootURI,
		Capabilities: clientCaps{
			TextDocument: textDocCaps{
				Synchronization: syncCaps{DidSave: true},
				PublishDiags:    publishDiagsCaps{RelatedInformation: true},
			},
			Workspace: workspaceCaps{ApplyEdit: false},
		},
		InitOptions: c.config.InitOptions,
	}

	_, err = c.request(initCtx, "initialize", params)
	if err != nil {
		c.setState(StateError)
		return fmt.Errorf("initialize: %w", err)
	}

	// Send initialized notification
	c.notify("initialized", struct{}{})

	c.setState(StateReady)
	logger.Info().Str("component", "lsp").Str("server", c.config.Name).Msg("lsp.ready")
	return nil
}

// Stop gracefully shuts down the LSP server.
func (c *Client) Stop(ctx context.Context) {
	if c.state == StateStopped || c.state == StateUnstarted {
		return
	}

	// Close all open files
	c.closeAllFiles(ctx)

	// Send shutdown request (5s timeout)
	shutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, _ = c.request(shutCtx, "shutdown", nil)

	// Send exit notification
	c.notify("exit", nil)

	// Wait for process or kill
	done := make(chan struct{})
	go func() {
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Wait()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
	}

	c.setState(StateStopped)
	logger.Info().Str("component", "lsp").Str("server", c.config.Name).Msg("lsp.stopped")
}

// ---------------------------------------------------------------------------
// Document lifecycle
// ---------------------------------------------------------------------------

// OpenFile notifies the server that a file was opened.
func (c *Client) OpenFile(ctx context.Context, path string) error {
	if c.State() != StateReady {
		return fmt.Errorf("server not ready")
	}

	uri := FileToURI(path)
	c.openFilesMu.Lock()
	if _, ok := c.openFiles[uri]; ok {
		c.openFilesMu.Unlock()
		return nil // already open
	}
	c.openFiles[uri] = 1
	c.openFilesMu.Unlock()

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	c.notify("textDocument/didOpen", didOpenParams{
		TextDocument: textDocItem{
			URI:        uri,
			LanguageID: DetectLanguage(path),
			Version:    1,
			Text:       string(content),
		},
	})
	return nil
}

// NotifyChange tells the server a file's content changed.
// Reads the new content from disk and sends full document replacement.
func (c *Client) NotifyChange(ctx context.Context, path string) error {
	if c.State() != StateReady {
		return fmt.Errorf("server not ready")
	}

	uri := FileToURI(path)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	// Increment version
	c.openFilesMu.Lock()
	ver := c.openFiles[uri] + 1
	c.openFiles[uri] = ver
	c.openFilesMu.Unlock()

	c.notify("textDocument/didChange", didChangeParams{
		TextDocument: versionedTextDocID{URI: uri, Version: ver},
		ContentChanges: []contentChange{{Text: string(content)}},
	})
	return nil
}

func (c *Client) closeAllFiles(ctx context.Context) {
	c.openFilesMu.Lock()
	uris := make([]string, 0, len(c.openFiles))
	for uri := range c.openFiles {
		uris = append(uris, uri)
	}
	c.openFiles = make(map[string]int)
	c.openFilesMu.Unlock()

	for _, uri := range uris {
		c.notify("textDocument/didClose", didCloseParams{
			TextDocument: textDocID{URI: uri},
		})
	}
}

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------

// GetDiagnostics returns all cached diagnostics, optionally filtered by file.
func (c *Client) GetDiagnostics(file string) []Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	if file != "" {
		uri := FileToURI(file)
		return c.diagnostics[uri]
	}

	var all []Diagnostic
	for _, diags := range c.diagnostics {
		all = append(all, diags...)
	}
	return all
}

// GetDiagnosticCounts returns aggregated counts across all files.
func (c *Client) GetDiagnosticCounts() DiagnosticCounts {
	return CountDiagnostics(c.GetDiagnostics(""))
}

// FindReferences queries the server for all references to a symbol.
func (c *Client) FindReferences(ctx context.Context, path string, line, col int) ([]Location, error) {
	if c.State() != StateReady {
		return nil, fmt.Errorf("server not ready")
	}

	// Ensure file is open
	_ = c.OpenFile(ctx, path)

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.request(reqCtx, "textDocument/references", referencesParams{
		TextDocument: textDocID{URI: FileToURI(path)},
		Position:     position{Line: line - 1, Character: col - 1}, // 1-based → 0-based
		Context:      referencesCtx{IncludeDeclaration: true},
	})
	if err != nil {
		return nil, err
	}

	var rawLocs []rawLocation
	if err := json.Unmarshal(result, &rawLocs); err != nil {
		return nil, fmt.Errorf("parse references: %w", err)
	}

	locs := make([]Location, 0, len(rawLocs))
	for _, rl := range rawLocs {
		locs = append(locs, Location{
			File:      URIToFile(rl.URI),
			StartLine: rl.Range.Start.Line + 1,
			StartCol:  rl.Range.Start.Character + 1,
			EndLine:   rl.Range.End.Line + 1,
			EndCol:    rl.Range.End.Character + 1,
		})
	}
	return locs, nil
}

// WaitForDiagnostics blocks until diagnostics arrive or timeout.
func (c *Client) WaitForDiagnostics(ctx context.Context, timeout time.Duration) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	initial := c.GetDiagnosticCounts().Total()
	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			return
		case <-ticker.C:
			if c.GetDiagnosticCounts().Total() != initial {
				// Diagnostics changed — give server a moment to finish
				time.Sleep(500 * time.Millisecond)
				return
			}
		}
	}
}

// HandlesFile returns true if this server handles the given file.
func (c *Client) HandlesFile(path string) bool {
	ext := strings.ToLower("." + strings.TrimPrefix(strings.ToLower(path[strings.LastIndex(path, ".")+1:]), "."))
	// Normalize: path might not have extension
	if !strings.Contains(path, ".") {
		return false
	}
	for _, ft := range c.config.FileTypes {
		if ft == ext {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 transport
// ---------------------------------------------------------------------------

func (c *Client) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := int(c.nextID.Add(1))

	ch := make(chan json.RawMessage, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  params,
	}
	if err := c.writeMessage(msg); err != nil {
		return nil, err
	}

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("server closed")
	}
}

func (c *Client) notify(method string, params any) {
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	_ = c.writeMessage(msg)
}

func (c *Client) writeMessage(msg jsonrpcMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	_, err = fmt.Fprint(c.stdin, header+string(body))
	return err
}

func (c *Client) readLoop() {
	defer close(c.done)

	for {
		body, err := c.readMessage()
		if err != nil {
			if c.State() != StateStopped {
				logger.Debug().Str("component", "lsp").Str("server", c.config.Name).Err(err).Msg("lsp.read_error")
			}
			return
		}

		var msg jsonrpcMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			continue
		}

		// Response to our request
		if msg.ID != nil && msg.Method == "" {
			c.pendingMu.Lock()
			if ch, ok := c.pending[*msg.ID]; ok {
				resultBytes, _ := json.Marshal(msg.Result)
				ch <- json.RawMessage(resultBytes)
			}
			c.pendingMu.Unlock()
			continue
		}

		// Server notification
		if msg.Method == "textDocument/publishDiagnostics" {
			c.handleDiagnostics(body)
		}
	}
}

func (c *Client) readMessage() ([]byte, error) {
	// Read headers until empty line
	contentLen := 0
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLen, _ = strconv.Atoi(val)
		}
	}
	if contentLen == 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLen)
	_, err := io.ReadFull(c.stdout, body)
	return body, err
}

func (c *Client) handleDiagnostics(body []byte) {
	// Extract params from the notification
	var envelope struct {
		Params publishDiagnosticsParams `json:"params"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return
	}

	params := envelope.Params
	diags := make([]Diagnostic, 0, len(params.Diagnostics))
	for _, raw := range params.Diagnostics {
		diags = append(diags, ConvertRawDiag(params.URI, raw))
	}

	c.diagnosticsMu.Lock()
	c.diagnostics[params.URI] = diags
	c.diagnosticsMu.Unlock()

	if c.onDiagnostics != nil {
		c.onDiagnostics(c.config.Name, c.GetDiagnosticCounts())
	}

	logger.Debug().Str("component", "lsp").
		Str("server", c.config.Name).
		Int("count", len(diags)).
		Str("uri", params.URI).
		Msg("lsp.diagnostics")
}
