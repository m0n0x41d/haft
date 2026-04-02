// Package jsonrpc implements JSON-RPC 2.0 over stdio.
// L5 in the functional hierarchy — transport only, no domain semantics.
package jsonrpc

import "encoding/json"

// Message is the wire format. Exactly one of Method/Result/Error is set.
type Message struct {
	JSONRPC string           `json:"jsonrpc"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *ErrorObj        `json:"error,omitempty"`
	ID      *int             `json:"id,omitempty"`
}

// ErrorObj is the JSON-RPC error object.
type ErrorObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// IsNotification returns true if the message has no ID (fire-and-forget).
func (m *Message) IsNotification() bool {
	return m.ID == nil && m.Method != ""
}

// IsRequest returns true if the message has a method AND an ID (expects response).
func (m *Message) IsRequest() bool {
	return m.ID != nil && m.Method != ""
}

// IsResponse returns true if the message has an ID but no method.
func (m *Message) IsResponse() bool {
	return m.ID != nil && m.Method == ""
}

// UnmarshalParams decodes the params into the given target.
func (m *Message) UnmarshalParams(v any) error {
	return json.Unmarshal(m.Params, v)
}

// NewNotification creates a notification (no response expected).
func NewNotification(method string, params any) Message {
	p, _ := json.Marshal(params)
	return Message{
		JSONRPC: "2.0",
		Method:  method,
		Params:  p,
	}
}

// NewRequest creates a request (expects response).
func NewRequest(id int, method string, params any) Message {
	p, _ := json.Marshal(params)
	return Message{
		JSONRPC: "2.0",
		Method:  method,
		Params:  p,
		ID:      &id,
	}
}

// NewResponse creates a success response.
func NewResponse(id int, result any) Message {
	r, _ := json.Marshal(result)
	return Message{
		JSONRPC: "2.0",
		Result:  r,
		ID:      &id,
	}
}

// NewError creates an error response.
func NewError(id int, code int, message string) Message {
	return Message{
		JSONRPC: "2.0",
		Error:   &ErrorObj{Code: code, Message: message},
		ID:      &id,
	}
}
