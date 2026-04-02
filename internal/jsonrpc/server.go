package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/m0n0x41d/haft/logger"
)

// Server reads JSON-RPC messages from a reader and writes to a writer.
// Newline-delimited JSON framing (one message per line).
// Thread-safe for concurrent writes.
type Server struct {
	reader  *bufio.Scanner
	writer  io.Writer
	writeMu sync.Mutex

	// Pending requests: id → response channel
	pending   map[int]chan Message
	pendingMu sync.Mutex
	nextID    int

	// Handler for incoming requests/notifications from the TUI
	handler func(msg Message)
}

// NewServer creates a JSON-RPC server on the given reader/writer pair.
func NewServer(r io.Reader, w io.Writer) *Server {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB max message
	return &Server{
		reader:  scanner,
		writer:  w,
		pending: make(map[int]chan Message),
	}
}

// SetHandler sets the callback for incoming TUI messages (requests + notifications).
func (s *Server) SetHandler(fn func(msg Message)) {
	s.handler = fn
}

// Send writes a notification to the TUI. Non-blocking.
func (s *Server) Send(method string, params any) error {
	return s.write(NewNotification(method, params))
}

// Request sends a request to the TUI and blocks until a response is received.
func (s *Server) Request(method string, params any) (json.RawMessage, error) {
	s.pendingMu.Lock()
	id := s.nextID
	s.nextID++
	ch := make(chan Message, 1)
	s.pending[id] = ch
	s.pendingMu.Unlock()

	logger.Debug().Str("component", "jsonrpc").Str("dir", "out").Str("type", "request").Int("id", id).Str("method", method).Msg("rpc.write")

	if err := s.write(NewRequest(id, method, params)); err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, err
	}

	resp := <-ch
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	return resp.Result, nil
}

// Respond sends a response to a TUI request.
func (s *Server) Respond(id int, result any) error {
	logger.Debug().Str("component", "jsonrpc").Str("dir", "out").Str("type", "response").Int("id", id).Msg("rpc.write")
	return s.write(NewResponse(id, result))
}

// RespondError sends an error response to a TUI request.
func (s *Server) RespondError(id int, code int, message string) error {
	logger.Warn().Str("component", "jsonrpc").Str("dir", "out").Str("type", "error_response").Int("id", id).Int("code", code).Str("message", message).Msg("rpc.write")
	return s.write(NewError(id, code, message))
}

// ReadLoop reads messages from the TUI until EOF or error.
// Blocks. Call in a goroutine.
func (s *Server) ReadLoop() error {
	for s.reader.Scan() {
		line := s.reader.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			logger.Warn().Str("component", "jsonrpc").Str("dir", "in").Err(err).Msg("rpc.parse_error")
			continue
		}

		// Response to a pending request
		if msg.IsResponse() {
			respID := 0
			if msg.ID != nil {
				respID = *msg.ID
			}
			logger.Debug().Str("component", "jsonrpc").Str("dir", "in").Str("type", "response").Int("id", respID).Msg("rpc.read")

			s.pendingMu.Lock()
			ch, ok := s.pending[*msg.ID]
			if ok {
				delete(s.pending, *msg.ID)
			}
			s.pendingMu.Unlock()
			if ok {
				ch <- msg
			} else {
				logger.Warn().Str("component", "jsonrpc").Int("id", respID).Msg("rpc.orphan_response")
			}
			continue
		}

		// Incoming request or notification from TUI
		logger.Debug().Str("component", "jsonrpc").Str("dir", "in").Str("type", "incoming").Str("method", msg.Method).Msg("rpc.read")
		if s.handler != nil {
			s.handler(msg)
		}
	}

	// Close all pending requests on EOF
	s.pendingMu.Lock()
	for id, ch := range s.pending {
		logger.Warn().Str("component", "jsonrpc").Int("id", id).Msg("rpc.pending_closed_on_eof")
		close(ch)
		delete(s.pending, id)
	}
	s.pendingMu.Unlock()

	return s.reader.Err()
}

func (s *Server) write(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = fmt.Fprintf(s.writer, "%s\n", data)
	return err
}
