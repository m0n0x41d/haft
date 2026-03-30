package agent

import "fmt"

// ---------------------------------------------------------------------------
// Subagent definitions — pure L2 types and validation.
// No I/O, no context.Context. Fully testable.
// ---------------------------------------------------------------------------

// SubagentDef defines a subagent's behavior, tools, and constraints.
type SubagentDef struct {
	Name         string   // display name (e.g., "explore", "title", "compact")
	Description  string   // for LLM tool description
	SystemPrompt string   // role-specific instructions
	AllowedTools []string // tool whitelist (empty = all tools)
	MaxSteps     int      // max ReAct loop iterations (default 30)
	ReadOnly     bool     // if true, write/edit excluded even if in AllowedTools
	Model        string   // override model (empty = use parent's model)
	Hidden       bool     // hidden subagents (title, compact) are not exposed to the LLM
}

const (
	// MaxSubagentDepth prevents recursive spawning.
	// Subagents cannot spawn sub-subagents.
	MaxSubagentDepth = 1

	// MaxConcurrentSubagents limits parallel goroutines.
	MaxConcurrentSubagents = 6

	// DefaultSubagentMaxSteps is the default safety limit.
	DefaultSubagentMaxSteps = 30
)

// ValidateSpawnDepth checks if the parent session is allowed to spawn children.
// Returns nil if allowed, error if depth limit exceeded.
// Pure function.
func ValidateSpawnDepth(parentID string) error {
	if parentID != "" {
		return fmt.Errorf("subagents cannot spawn sub-subagents (depth limit %d)", MaxSubagentDepth)
	}
	return nil
}

// BuiltinSubagents returns the built-in subagent definitions.
func BuiltinSubagents() map[string]SubagentDef {
	return map[string]SubagentDef{
		"explore": ExploreSubagent(),
		"title":   TitleSubagent(),
		"compact": CompactSubagent(),
	}
}

// ExploreSubagent returns the read-only codebase investigation subagent.
// Used for parallel investigation of large repos.
func ExploreSubagent() SubagentDef {
	return SubagentDef{
		Name:        "explore",
		Description: "Fast, read-only codebase exploration. Spawns a parallel agent that can search files, read code, and run read-only commands. Returns a compressed summary of findings.",
		SystemPrompt: `You are an explore subagent — a fast, read-only codebase investigator.

## Your job
Search, read, and analyze code to answer the question you were given. Be thorough but focused.

## Rules
- You are READ-ONLY. You cannot write, edit, or modify any files.
- Be concise in your final answer — your output is a summary returned to the parent agent.
- Focus on facts: file paths, function signatures, line numbers, code snippets.
- If you can't find what you're looking for, say so — don't guess.
- You have a limited step budget. Prioritize the most informative searches first.`,
		AllowedTools: []string{"read", "glob", "grep", "bash"},
		MaxSteps:     DefaultSubagentMaxSteps,
		ReadOnly:     true,
	}
}

// TitleSubagent returns the hidden session title generator.
func TitleSubagent() SubagentDef {
	return SubagentDef{
		Name:         "title",
		Description:  "Generate a short session title",
		SystemPrompt: "", // uses BuildTitlePrompt
		AllowedTools: nil,
		MaxSteps:     1,
		Hidden:       true,
	}
}

// CompactSubagent returns the hidden context compaction summarizer.
func CompactSubagent() SubagentDef {
	return SubagentDef{
		Name:        "compact",
		Description: "Summarize conversation context for compaction",
		SystemPrompt: `You are a context compaction agent. Summarize the conversation so far into a brief, information-dense summary that preserves:
- What problem is being solved
- What approaches were considered and why
- What was implemented so far
- What files were modified
- What remains to be done

Keep it under 500 words. Be factual, not conversational.`,
		AllowedTools: nil,
		MaxSteps:     1,
		Hidden:       true,
	}
}

// SubagentDefByName looks up a subagent definition.
// Returns the def and true if found, zero value and false otherwise.
// Pure function.
func SubagentDefByName(name string) (SubagentDef, bool) {
	defs := BuiltinSubagents()
	def, ok := defs[name]
	return def, ok
}

// VisibleSubagents returns only non-hidden subagent definitions.
// Used to build the spawn_agent tool schema (LLM shouldn't see hidden agents).
// Pure function.
func VisibleSubagents() []SubagentDef {
	var visible []SubagentDef
	for _, def := range BuiltinSubagents() {
		if !def.Hidden {
			visible = append(visible, def)
		}
	}
	return visible
}
