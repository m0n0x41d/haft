// Package lsp provides a minimal LSP client for language server integration.
//
// Architecture (func-arch):
//
//	L0: types.go   — protocol types, diagnostics, server state (pure data)
//	L1: format.go  — diagnostic formatting, language detection (pure functions)
//	L2: client.go  — JSON-RPC 2.0 over stdio (I/O boundary)
//	L3: manager.go — server lifecycle, lazy startup (orchestration)
package lsp

// ---------------------------------------------------------------------------
// L0: Pure data types
// ---------------------------------------------------------------------------

// ServerState tracks the lifecycle of an LSP server process.
type ServerState int

const (
	StateUnstarted ServerState = iota
	StateStarting
	StateReady
	StateError
	StateStopped
)

func (s ServerState) String() string {
	switch s {
	case StateUnstarted:
		return "unstarted"
	case StateStarting:
		return "starting"
	case StateReady:
		return "ready"
	case StateError:
		return "error"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// ServerConfig describes how to start and communicate with an LSP server.
type ServerConfig struct {
	Name        string            // human name (e.g., "gopls")
	Command     string            // executable name
	Args        []string          // CLI arguments
	Env         map[string]string // extra environment variables
	FileTypes   []string          // handled extensions (e.g., [".go"])
	RootMarkers []string          // project root indicators (e.g., ["go.mod"])
	InitOptions map[string]any    // LSP initializationOptions
	Settings    map[string]any    // workspace/configuration settings
}

// Diagnostic represents a single LSP diagnostic.
type Diagnostic struct {
	File     string
	Line     int // 1-based
	Col      int // 1-based
	Severity DiagnosticSeverity
	Source   string
	Code     string
	Message  string
	Tags     []string // "unnecessary", "deprecated"
}

// DiagnosticSeverity mirrors LSP DiagnosticSeverity (1=Error, 2=Warning, 3=Info, 4=Hint).
type DiagnosticSeverity int

const (
	SeverityError   DiagnosticSeverity = 1
	SeverityWarning DiagnosticSeverity = 2
	SeverityInfo    DiagnosticSeverity = 3
	SeverityHint    DiagnosticSeverity = 4
)

// DiagnosticCounts aggregates diagnostic counts by severity.
type DiagnosticCounts struct {
	Error   int
	Warning int
	Info    int
	Hint    int
}

// Total returns the sum of all diagnostics.
func (d DiagnosticCounts) Total() int {
	return d.Error + d.Warning + d.Info + d.Hint
}

// Location describes a position in a file (from references, definitions).
type Location struct {
	File      string
	StartLine int // 1-based
	StartCol  int // 1-based
	EndLine   int // 1-based
	EndCol    int // 1-based
}

// ---------------------------------------------------------------------------
// LSP protocol message types (minimal subset for JSON-RPC communication)
// ---------------------------------------------------------------------------

// jsonrpcMessage is the wire format for JSON-RPC 2.0.
type jsonrpcMessage struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      *int      `json:"id,omitempty"`
	Method  string    `json:"method,omitempty"`
	Params  any       `json:"params,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// initializeParams mirrors LSP InitializeParams.
type initializeParams struct {
	ProcessID    int        `json:"processId"`
	RootURI      string     `json:"rootUri"`
	Capabilities clientCaps `json:"capabilities"`
	InitOptions  any        `json:"initializationOptions,omitempty"`
}

type clientCaps struct {
	TextDocument textDocCaps   `json:"textDocument"`
	Workspace    workspaceCaps `json:"workspace,omitempty"`
}

type textDocCaps struct {
	Synchronization syncCaps         `json:"synchronization"`
	PublishDiags    publishDiagsCaps `json:"publishDiagnostics"`
}

type syncCaps struct {
	DidSave bool `json:"didSave"`
}

type publishDiagsCaps struct {
	RelatedInformation bool `json:"relatedInformation"`
}

type workspaceCaps struct {
	ApplyEdit bool `json:"applyEdit"`
}

// didOpenParams mirrors textDocument/didOpen params.
type didOpenParams struct {
	TextDocument textDocItem `json:"textDocument"`
}

type textDocItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// didChangeParams mirrors textDocument/didChange params.
type didChangeParams struct {
	TextDocument   versionedTextDocID `json:"textDocument"`
	ContentChanges []contentChange    `json:"contentChanges"`
}

type versionedTextDocID struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type contentChange struct {
	Text string `json:"text"` // full document replacement
}

// didCloseParams mirrors textDocument/didClose params.
type didCloseParams struct {
	TextDocument textDocID `json:"textDocument"`
}

type textDocID struct {
	URI string `json:"uri"`
}

// referencesParams mirrors textDocument/references params.
type referencesParams struct {
	TextDocument textDocID     `json:"textDocument"`
	Position     position      `json:"position"`
	Context      referencesCtx `json:"context"`
}

type position struct {
	Line      int `json:"line"`      // 0-based
	Character int `json:"character"` // 0-based
}

type referencesCtx struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// publishDiagnosticsParams is the server→client notification payload.
type publishDiagnosticsParams struct {
	URI         string    `json:"uri"`
	Diagnostics []rawDiag `json:"diagnostics"`
}

type rawDiag struct {
	Range    rawRange `json:"range"`
	Severity int      `json:"severity"`
	Code     any      `json:"code,omitempty"` // string or int
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
	Tags     []int    `json:"tags,omitempty"` // 1=unnecessary, 2=deprecated
}

type rawRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

// rawLocation is the server response format for references.
type rawLocation struct {
	URI   string   `json:"uri"`
	Range rawRange `json:"range"`
}
