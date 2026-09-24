package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/m0n0x41d/haft/internal/core/app"
	"github.com/m0n0x41d/haft/internal/core/delivery"
)

// Server keeps transport state only. Execute captures fresh application state on
// every call, including when another persistent client has just published a write.
type Server struct {
	Service app.Service
	Version string
}
type envelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func (s Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	enc := json.NewEncoder(out)
	initialized := false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw, err := readFrame(reader)
		if errors.Is(err, io.EOF) && len(raw) == 0 {
			return nil
		}
		if err != nil && !errors.Is(err, io.EOF) {
			if errors.Is(err, errFrameLimit) {
				if e := enc.Encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, boundedDiagnostic(err.Error())}}); e != nil {
					return e
				}
				continue
			}
			return err
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var request envelope
		if err := Decode(raw, &request); err != nil {
			if e := enc.Encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, boundedDiagnostic(err.Error())}}); e != nil {
				return e
			}
			continue
		}
		id := request.ID
		if len(id) == 0 { // Notifications have no effects and never invoke Execute.
			continue
		}
		var idValue any
		_ = json.Unmarshal(id, &idValue)
		_, stringID := idValue.(string)
		_, numberID := idValue.(float64)
		if request.JSONRPC != "2.0" || (!stringID && !numberID) || len(id) > 256 {
			if e := enc.Encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32600, "Invalid JSON-RPC request or id"}}); e != nil {
				return e
			}
			continue
		}
		result, rpcErr := s.dispatch(ctx, request, &initialized)
		if rpcErr != nil {
			rpcErr.Message = boundedDiagnostic(rpcErr.Message)
		}
		if e := enc.Encode(response{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr}); e != nil {
			return e
		}
	}
}

var errFrameLimit = errors.New("input_limit: MCP frame exceeds maximum bytes")

func readFrame(r *bufio.Reader) ([]byte, error) {
	var all []byte
	tooLarge := false
	for {
		part, err := r.ReadSlice('\n')
		if !tooLarge {
			if len(all)+len(part) > MaxInputBytes {
				tooLarge = true
				all = nil
			} else {
				all = append(all, part...)
			}
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if tooLarge {
			return nil, errFrameLimit
		}
		return all, err
	}
}
func (s Server) dispatch(ctx context.Context, q envelope, initialized *bool) (any, *rpcError) {
	invalid := func(err error) (any, *rpcError) { return nil, &rpcError{-32602, err.Error()} }
	switch q.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string         `json:"protocolVersion"`
			Capabilities    map[string]any `json:"capabilities"`
			ClientInfo      struct {
				Name        string           `json:"name"`
				Version     string           `json:"version"`
				Title       string           `json:"title,omitempty"`
				Description string           `json:"description,omitempty"`
				WebsiteURL  string           `json:"websiteUrl,omitempty"`
				Icons       []map[string]any `json:"icons,omitempty"`
			} `json:"clientInfo"`
			Meta map[string]any `json:"_meta,omitempty"`
		}
		if err := Decode(q.Params, &p); err != nil {
			return invalid(err)
		}
		if p.ProtocolVersion == "" || p.ClientInfo.Name == "" {
			return invalid(fmt.Errorf("protocolVersion and clientInfo.name are required"))
		}
		version := p.ProtocolVersion
		switch version {
		case "2024-11-05", "2025-03-26", "2025-06-18":
		default:
			version = "2025-06-18"
		}
		*initialized = true
		return map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{"listChanged": false}}, "serverInfo": map[string]any{"name": "haft10", "version": s.Version}, "instructions": delivery.Guide}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		if !*initialized {
			return nil, &rpcError{-32002, "Initialize the server first"}
		}
		if len(q.Params) > 0 {
			var p struct {
				Cursor string         `json:"cursor,omitempty"`
				Meta   map[string]any `json:"_meta,omitempty"`
			}
			if err := Decode(q.Params, &p); err != nil {
				return invalid(err)
			}
			if p.Cursor != "" {
				return invalid(fmt.Errorf("unsupported cursor"))
			}
		}
		return map[string]any{"tools": []any{map[string]any{"name": "haft", "description": "Read/write project records, retrieve source, navigate Go, prepare/observe checks and preview/apply changes. " + delivery.Guide + " Explicit writes are trusted local inputs, not operator attestation.", "inputSchema": RequestSchema(), "annotations": map[string]any{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": false, "openWorldHint": false}}}}, nil
	case "tools/call":
		if !*initialized {
			return nil, &rpcError{-32002, "Initialize the server first"}
		}
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
			Meta      map[string]any  `json:"_meta,omitempty"`
		}
		if err := Decode(q.Params, &p); err != nil {
			return invalid(err)
		}
		if p.Name != "haft" {
			return invalid(fmt.Errorf("unknown tool %q", p.Name))
		}
		request, err := DecodeRequest(p.Arguments)
		if err != nil {
			return invalid(err)
		}
		result := s.Service.Call(ctx, request)
		return delivery.MCPResult(result), nil
	default:
		return nil, &rpcError{-32601, "Method not found"}
	}
}
