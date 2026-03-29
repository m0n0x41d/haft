package agent

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Compaction — pure functions for context window management.
// Three stages: observation masking → LLM summarization → emergency truncation.
// ---------------------------------------------------------------------------

const (
	// Stage1Threshold: trigger observation masking when usage exceeds this fraction.
	Stage1Threshold = 0.70
	// Stage2Threshold: trigger LLM compaction when usage exceeds this fraction.
	Stage2Threshold = 0.85
	// ProtectLastN is the number of recent messages to never compact.
	ProtectLastN = 6
)

// CompactionStage determines which compaction stage to trigger.
// Pure function.
func CompactionStage(used, limit int) int {
	if limit <= 0 {
		return 0
	}
	ratio := float64(used) / float64(limit)
	switch {
	case ratio >= Stage2Threshold:
		return 2
	case ratio >= Stage1Threshold:
		return 1
	default:
		return 0
	}
}

// PruneOldToolOutputs removes tool output content from old messages
// while preserving the structure (tool name, call ID). Stage 1: free, no LLM.
// Pure function — returns new slice, doesn't mutate input.
func PruneOldToolOutputs(messages []Message, protectLastN int) []Message {
	if len(messages) <= protectLastN {
		return messages // nothing to prune
	}

	pruned := make([]Message, len(messages))
	copy(pruned, messages)

	cutoff := len(pruned) - protectLastN

	for i := 0; i < cutoff; i++ {
		msg := pruned[i]
		if msg.Role != RoleTool && msg.Role != RoleAssistant {
			continue
		}

		var newParts []Part
		for _, p := range msg.Parts {
			switch tp := p.(type) {
			case ToolResultPart:
				// Keep tool identity, remove output
				newParts = append(newParts, ToolResultPart{
					ToolCallID: tp.ToolCallID,
					ToolName:   tp.ToolName,
					Content:    "[output pruned for context management]",
					IsError:    tp.IsError,
				})
			case TextPart:
				// Truncate long assistant text from old turns
				text := tp.Text
				if len(text) > 500 {
					text = text[:500] + "... [truncated]"
				}
				newParts = append(newParts, TextPart{Text: text})
			default:
				newParts = append(newParts, p)
			}
		}
		pruned[i].Parts = newParts
	}

	return pruned
}

// BuildArtifactAnchor constructs the artifact sections of the compaction anchor.
// These sections are read directly from .quint/ data — no LLM summarization.
// Pure function — takes pre-fetched artifact summaries as input.
func BuildArtifactAnchor(problemSummary, decisionSummary, portfolioSummary string) string {
	var b strings.Builder
	b.WriteString("# Context Anchor (preserved across compaction)\n\n")

	if problemSummary != "" {
		b.WriteString("## Problem\n")
		b.WriteString(problemSummary)
		b.WriteString("\n\n")
	}

	if portfolioSummary != "" {
		b.WriteString("## Exploration\n")
		b.WriteString(portfolioSummary)
		b.WriteString("\n\n")
	}

	if decisionSummary != "" {
		b.WriteString("## Decision\n")
		b.WriteString(decisionSummary)
		b.WriteString("\n\n")
	}

	return b.String()
}

// BuildCompactPrompt creates the prompt for the compact subagent.
// The subagent receives artifact anchor + conversation to summarize.
// Pure function.
func BuildCompactPrompt(artifactAnchor string, existingAnchor string, conversationText string) string {
	var b strings.Builder

	b.WriteString("You are performing context compaction for a lemniscate engineering agent.\n\n")

	if artifactAnchor != "" {
		b.WriteString("## Artifact context (already preserved — do NOT repeat these)\n\n")
		b.WriteString(artifactAnchor)
		b.WriteString("\n")
	}

	if existingAnchor != "" {
		b.WriteString("## Previous compaction summary (merge new information into this)\n\n")
		b.WriteString(existingAnchor)
		b.WriteString("\n")
	}

	b.WriteString("## Conversation to summarize\n\n")
	b.WriteString(conversationText)
	b.WriteString("\n\n")

	b.WriteString(`## Your task

Produce a structured summary with these sections:

### Implementation Progress
- List ALL file paths with line numbers that were read or modified (VERBATIM — never paraphrase paths)
- List commands run and their outcomes (pass/fail)
- Note errors encountered and how they were resolved

### Working Context
- What was the agent doing most recently?
- What approach was being taken and why?
- What remains to be done? (specific, actionable next steps)
- Any blockers, gotchas, or failed approaches to avoid?

CRITICAL RULES:
- Preserve ALL file paths exactly as they appear (e.g., src/auth/token.go:42)
- Do NOT repeat information from the artifact context above
- Be factual and dense — this is a handoff brief, not a conversation
- If merging with a previous summary, UPDATE sections rather than appending duplicate information
- Keep total length under 1500 words`)

	return b.String()
}

// BuildCompactedHistory replaces old messages with an anchor system message.
// Keeps: system prompt (index 0) + anchor message + last N messages.
// Pure function.
func BuildCompactedHistory(
	fullHistory []Message,
	anchorText string,
	protectLastN int,
) []Message {
	if len(fullHistory) <= protectLastN+1 {
		return fullHistory
	}

	var result []Message

	// Keep system prompt (first message)
	if len(fullHistory) > 0 && fullHistory[0].Role == RoleSystem {
		result = append(result, fullHistory[0])
	}

	// Insert anchor as system message
	result = append(result, Message{
		Role:  RoleSystem,
		Parts: []Part{TextPart{Text: fmt.Sprintf("[Context compaction summary]\n\n%s", anchorText)}},
	})

	// Keep last N messages
	start := len(fullHistory) - protectLastN
	if start < 0 {
		start = 0
	}
	result = append(result, fullHistory[start:]...)

	return result
}

// ExtractConversationText extracts readable text from messages for summarization.
// Skips system messages and heavily truncated tool outputs.
// Pure function.
func ExtractConversationText(messages []Message, skipLastN int) string {
	var b strings.Builder
	end := len(messages) - skipLastN
	if end < 0 {
		end = 0
	}

	for _, msg := range messages[:end] {
		if msg.Role == RoleSystem {
			continue
		}

		switch msg.Role {
		case RoleUser:
			b.WriteString("User: ")
			b.WriteString(msg.Text())
			b.WriteString("\n\n")
		case RoleAssistant:
			text := msg.Text()
			if text != "" {
				b.WriteString("Assistant: ")
				if len(text) > 1000 {
					text = text[:1000] + "..."
				}
				b.WriteString(text)
				b.WriteString("\n")
			}
			for _, tc := range msg.ToolCalls() {
				b.WriteString(fmt.Sprintf("  [tool: %s]\n", tc.ToolName))
			}
			b.WriteString("\n")
		case RoleTool:
			for _, p := range msg.Parts {
				if tr, ok := p.(ToolResultPart); ok {
					status := "ok"
					if tr.IsError {
						status = "error"
					}
					content := tr.Content
					if len(content) > 200 {
						content = content[:200] + "..."
					}
					b.WriteString(fmt.Sprintf("  [%s %s]: %s\n", tr.ToolName, status, content))
				}
			}
		}
	}

	return b.String()
}
