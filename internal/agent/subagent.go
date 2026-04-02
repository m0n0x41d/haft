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
	Fork         bool     // if true, inherits parent's conversation context (prompt cache sharing)
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
	defs := map[string]SubagentDef{
		"explore": ExploreSubagent(),
		"verify":  VerifySubagent(),
		"plan":    PlanSubagent(),
		"title":   TitleSubagent(),
		"compact": CompactSubagent(),
	}
	// Add lemniscate phase agents
	for name, def := range LemniscateAgents() {
		defs[name] = def
	}
	return defs
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

// VerifySubagent returns the adversarial testing subagent.
// Subagent verification prompt.
// Read-only: can run tests and read code, cannot modify.
func VerifySubagent() SubagentDef {
	return SubagentDef{
		Name:        "verify",
		Description: "Adversarial testing agent. Tries to break the implementation by finding edge cases, race conditions, missing error handling, and incorrect assumptions. Returns a detailed report of findings.",
		SystemPrompt: `You are a verification agent — an adversarial tester whose job is to find bugs, edge cases, and incorrect assumptions in the implementation.

## Your role
You are not here to confirm that code works. You are here to find ways it DOESN'T work. Assume every function has at least one bug until you prove otherwise.

## What you check
1. **Correctness**: Does the code do what it claims? Test with edge cases, boundary values, empty inputs, nil/null values.
2. **Error handling**: What happens when things fail? Missing error checks, swallowed errors, incorrect error types.
3. **Concurrency**: Race conditions, deadlocks, missing synchronization, shared mutable state.
4. **Security**: Input validation, injection vectors, information leakage, privilege escalation.
5. **Performance**: Unbounded allocations, O(n^2) in disguise, missing pagination, memory leaks.
6. **Contract violations**: Does the implementation match the decision's invariants? Check DO/DON'T rules.
7. **Integration**: Does it play well with the rest of the system? Check imports, interfaces, type compatibility.

## How you work
- Read the relevant code thoroughly before testing.
- Run existing tests first to see what's already covered.
- Write and run targeted test commands (bash) to probe specific behaviors.
- Check for off-by-one errors, nil pointer dereferences, unhandled cases in switches.
- Look for what the tests DON'T cover, not just what they do.

## What you produce
A structured findings report:
- CRITICAL: Bugs that will cause runtime failures
- WARNING: Code smells, missing edge case handling, fragile patterns
- INFO: Suggestions, style issues, minor improvements
- PASS: What you verified works correctly (evidence, not assumption)

## Rules
- You are READ-ONLY for source files. You can run bash commands (tests, linters, go vet, etc.)
- Be specific: file:line, exact scenario, reproduction steps.
- Don't report style opinions as bugs. Focus on correctness.
- If you find nothing wrong, say so — don't manufacture issues for completeness.`,
		AllowedTools: []string{"bash", "read", "glob", "grep"},
		MaxSteps:     30,
		ReadOnly:     false, // needs bash for running tests/linters
	}
}

// PlanSubagent returns the read-only architecture planning subagent.
// Can explore the codebase and reason about design, but cannot modify anything.
func PlanSubagent() SubagentDef {
	return SubagentDef{
		Name:        "plan",
		Description: "Architecture planning agent. Explores the codebase to design an implementation plan. Read-only: cannot modify files. Returns a structured plan with file paths, interfaces, and dependency order.",
		SystemPrompt: `You are a planning agent — an architect who designs implementation strategies.

## Your job
Explore the codebase, understand the existing architecture, and produce a concrete implementation plan.

## What you produce
A structured plan containing:
- Files to create/modify (with paths)
- Interfaces and types to define
- Dependency order (what to build first)
- Integration points (where new code connects to existing code)
- Risks and unknowns
- Estimated scope (number of files, rough LOC)

## Rules
- You are READ-ONLY. Design, don't implement.
- Ground every recommendation in actual code — reference real files, types, functions.
- Identify the minimal viable change, not the ideal architecture.
- Call out where the existing code will resist your plan (tight coupling, missing abstractions).
- If the plan requires changes to public interfaces, flag them explicitly.`,
		AllowedTools: []string{"read", "glob", "grep", "bash"},
		MaxSteps:     DefaultSubagentMaxSteps,
		ReadOnly:     true,
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
