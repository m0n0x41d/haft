package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/protocol"
)

// QuestionAsker sends a question to the user via the protocol bus and blocks until answered.
type QuestionAsker interface {
	AskQuestion(p protocol.QuestionAsk) (protocol.QuestionReply, error)
}

// AskUserQuestionTool allows the agent to ask the user a question and wait for an answer.
// BLOCKING: the tool call does not return until the user responds.
type AskUserQuestionTool struct {
	asker QuestionAsker
}

func NewAskUserQuestionTool(asker QuestionAsker) *AskUserQuestionTool {
	return &AskUserQuestionTool{asker: asker}
}

func (t *AskUserQuestionTool) Name() string { return "ask_user_question" }

func (t *AskUserQuestionTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "ask_user_question",
		Description: `Ask the user a question and wait for their response. Use this when you need clarification, confirmation, or a decision from the user before proceeding.

Examples:
- "Should I proceed with approach A or B?"
- "What database should I target?"
- "The test is failing — should I fix it or skip it?"

If you provide options, the user picks from them. If not, they type a free-form answer.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"question": map[string]any{
					"type":        "string",
					"description": "The question to ask the user",
				},
				"options": map[string]any{
					"type":        "array",
					"description": "Optional list of choices (user picks one). Omit for free-form answer.",
					"items":       map[string]any{"type": "string"},
				},
			},
			"required": []string{"question"},
		},
	}
}

func (t *AskUserQuestionTool) Execute(_ context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Question == "" {
		return agent.ToolResult{}, fmt.Errorf("question is required")
	}

	reply, err := t.asker.AskQuestion(protocol.QuestionAsk{
		Question: args.Question,
		Options:  args.Options,
	})
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("ask question: %w", err)
	}

	return agent.PlainResult(fmt.Sprintf("User answered: %s", reply.Answer)), nil
}
