package provider

import (
	"context"

	"github.com/m0n0x41d/haft/internal/agent"
)

// StreamDelta carries one chunk of a streaming LLM response.
type StreamDelta struct {
	Text         string          // text content delta
	Thinking     string          // reasoning/thinking summary delta
	ToolCalls    []ToolCallDelta // tool call deltas (partial)
	Done         bool            // true on final chunk
	FinishReason string          // "" until done, then "stop" or "tool_calls"
}

// ToolCallDelta carries incremental data for a single tool call.
type ToolCallDelta struct {
	Index     int    // position in the tool_calls array
	ID        string // set on first delta for this index
	Name      string // set on first delta for this index
	ArgsDelta string // incremental JSON fragment
}

// LLMProvider abstracts LLM interaction.
// Implementations: OpenAI (MVP), Anthropic, Google, Ollama (future).
type LLMProvider interface {
	// Stream sends a conversation to the LLM and calls handler for each chunk.
	// The handler is called synchronously from the streaming goroutine.
	// Returns after the stream completes or ctx is canceled.
	Stream(
		ctx context.Context,
		messages []agent.Message,
		tools []agent.ToolSchema,
		handler func(StreamDelta),
	) (*agent.Message, error)

	// ModelID returns the model identifier (e.g. "gpt-4o").
	ModelID() string
}
