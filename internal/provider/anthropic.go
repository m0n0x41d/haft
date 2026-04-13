package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/m0n0x41d/haft/internal/agent"
)

// AnthropicProvider implements LLMProvider using the Anthropic Messages API.
type AnthropicProvider struct {
	client anthropic.Client
	model  string
}

var _ LLMProvider = (*AnthropicProvider)(nil)

// NewAnthropic creates an Anthropic provider.
// Resolves API key from: explicit param → ANTHROPIC_API_KEY env.
func NewAnthropic(model, apiKey string) (*AnthropicProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("no Anthropic API key: set ANTHROPIC_API_KEY or run 'haft setup'")
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &AnthropicProvider{
		client: client,
		model:  model,
	}, nil
}

func (p *AnthropicProvider) ModelID() string { return p.model }

// Stream sends conversation to Anthropic and streams the response.
func (p *AnthropicProvider) Stream(
	ctx context.Context,
	messages []agent.Message,
	tools []agent.ToolSchema,
	handler func(StreamDelta),
) (*agent.Message, error) {
	system, msgs := convertToAnthropicMessages(messages)
	anthropicTools := convertToAnthropicTools(tools)

	maxTokens := int64(16384)
	if m, ok := DefaultRegistry().Lookup(p.model); ok && m.DefaultMaxOut > 0 {
		maxTokens = int64(m.DefaultMaxOut)
	}

	params := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: maxTokens,
		Messages:  msgs,
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: system},
		}
	}
	if len(anthropicTools) > 0 {
		params.Tools = anthropicTools
	}

	// Extended thinking for reasoning models
	if m, ok := DefaultRegistry().Lookup(p.model); ok && m.CanReason {
		params.Thinking = anthropic.ThinkingConfigParamOfEnabled(maxTokens)
		params.MaxTokens = maxTokens * 2 // thinking needs headroom
	}

	stream := p.client.Messages.NewStreaming(ctx, params)

	var (
		textBuf         strings.Builder
		thinkBuf        strings.Builder
		toolCalls       []agent.ToolCallPart
		currentToolID   string
		currentToolName string
		currentToolArgs strings.Builder
		inputTokens     int64
		outputTokens    int64
	)

	for stream.Next() {
		event := stream.Current()

		switch event.Type {
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				currentToolID = event.ContentBlock.ID
				currentToolName = event.ContentBlock.Name
				currentToolArgs.Reset()
			}

		case "content_block_delta":
			switch event.Delta.Type {
			case "text_delta":
				textBuf.WriteString(event.Delta.Text)
				handler(StreamDelta{Text: event.Delta.Text})
			case "thinking_delta":
				thinkBuf.WriteString(event.Delta.Thinking)
				handler(StreamDelta{Thinking: event.Delta.Thinking})
			case "input_json_delta":
				currentToolArgs.WriteString(event.Delta.PartialJSON)
			}

		case "content_block_stop":
			if currentToolID != "" {
				toolCalls = append(toolCalls, agent.ToolCallPart{
					ToolCallID: currentToolID,
					ToolName:   currentToolName,
					Arguments:  currentToolArgs.String(),
				})
				currentToolID = ""
				currentToolName = ""
				currentToolArgs.Reset()
			}

		case "message_delta":
			if event.Usage.OutputTokens > 0 {
				outputTokens = event.Usage.OutputTokens
			}

		case "message_start":
			if event.Message.Usage.InputTokens > 0 {
				inputTokens = event.Message.Usage.InputTokens
			}
		}
	}

	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("anthropic stream: %w", err)
	}

	msg := &agent.Message{
		Role:      agent.RoleAssistant,
		Model:     p.model,
		CreatedAt: time.Now().UTC(),
		Tokens:    int(inputTokens + outputTokens),
	}
	if text := textBuf.String(); text != "" {
		msg.Parts = append(msg.Parts, agent.TextPart{Text: text})
	}
	for _, tc := range toolCalls {
		msg.Parts = append(msg.Parts, tc)
	}

	return msg, nil
}

// ---------------------------------------------------------------------------
// Converters: agent types → Anthropic API types
// ---------------------------------------------------------------------------

func convertToAnthropicMessages(messages []agent.Message) (string, []anthropic.MessageParam) {
	var system string
	var result []anthropic.MessageParam

	for _, msg := range messages {
		switch msg.Role {
		case agent.RoleSystem:
			system = msg.Text()

		case agent.RoleUser:
			blocks := userBlocks(msg)
			if len(blocks) > 0 {
				result = mergeOrAppend(result, "user", blocks)
			}

		case agent.RoleAssistant:
			blocks := assistantBlocks(msg)
			if len(blocks) > 0 {
				result = mergeOrAppend(result, "assistant", blocks)
			}

		case agent.RoleTool:
			blocks := toolResultBlocks(msg)
			if len(blocks) > 0 {
				result = mergeOrAppend(result, "user", blocks)
			}
		}
	}

	return system, result
}

func userBlocks(msg agent.Message) []anthropic.ContentBlockParamUnion {
	var blocks []anthropic.ContentBlockParamUnion
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case agent.TextPart:
			if p.Text != "" {
				blocks = append(blocks, anthropic.NewTextBlock(p.Text))
			}
		case agent.ImagePart:
			if len(p.Data) == 0 {
				continue
			}
			blocks = append(blocks, anthropic.NewImageBlockBase64(p.MIMEType, base64.StdEncoding.EncodeToString(p.Data)))
		case agent.ToolResultPart:
			blocks = append(blocks, anthropic.NewToolResultBlock(p.ToolCallID, p.Content, p.IsError))
		}
	}
	return blocks
}

func assistantBlocks(msg agent.Message) []anthropic.ContentBlockParamUnion {
	var blocks []anthropic.ContentBlockParamUnion
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case agent.TextPart:
			if p.Text != "" {
				blocks = append(blocks, anthropic.NewTextBlock(p.Text))
			}
		case agent.ToolCallPart:
			var input any
			_ = json.Unmarshal([]byte(p.Arguments), &input)
			if input == nil {
				input = map[string]any{}
			}
			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfToolUse: &anthropic.ToolUseBlockParam{
					ID:    p.ToolCallID,
					Type:  "tool_use",
					Name:  p.ToolName,
					Input: input,
				},
			})
		}
	}
	return blocks
}

func toolResultBlocks(msg agent.Message) []anthropic.ContentBlockParamUnion {
	var blocks []anthropic.ContentBlockParamUnion
	for _, part := range msg.Parts {
		if p, ok := part.(agent.ToolResultPart); ok {
			blocks = append(blocks, anthropic.NewToolResultBlock(p.ToolCallID, p.Content, p.IsError))
		}
	}
	return blocks
}

// mergeOrAppend adds blocks to the last message if same role, or creates new.
// Anthropic requires strictly alternating user/assistant turns.
func mergeOrAppend(msgs []anthropic.MessageParam, role string, blocks []anthropic.ContentBlockParamUnion) []anthropic.MessageParam {
	r := anthropic.MessageParamRole(role)
	if len(msgs) > 0 && msgs[len(msgs)-1].Role == r {
		msgs[len(msgs)-1].Content = append(msgs[len(msgs)-1].Content, blocks...)
		return msgs
	}
	return append(msgs, anthropic.MessageParam{
		Role:    r,
		Content: blocks,
	})
}

func convertToAnthropicTools(tools []agent.ToolSchema) []anthropic.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		schemaJSON, _ := json.Marshal(t.Parameters)
		var inputSchema anthropic.ToolInputSchemaParam
		_ = json.Unmarshal(schemaJSON, &inputSchema)
		inputSchema.Type = "object"

		result = append(result, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: inputSchema,
			},
		})
	}
	return result
}
