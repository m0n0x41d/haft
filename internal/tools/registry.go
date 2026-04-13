package tools

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/agent"
)

// ToolExecutor is the interface each tool implements.
type ToolExecutor interface {
	Name() string
	Schema() agent.ToolSchema
	Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error)
}

// CycleResolver provides active cycle state to tools for guardrail checks.
type CycleResolver func(ctx context.Context) *agent.Cycle

// CycleUpdater persists repaired cycle state after tool-side recovery.
type CycleUpdater func(ctx context.Context, cycle *agent.Cycle) error

// DecisionBoundaryChecker reports whether the compare -> decide boundary has
// an explicit post-compare user selection. It is only consulted before
// haft_decision(decide); compare remains callable without another user message.
// Returns true in autonomous mode (user explicitly delegated).
type DecisionBoundaryChecker func(ctx context.Context, cycle *agent.Cycle) (bool, error)

// Registry holds all available tools.
type Registry struct {
	tools                   map[string]ToolExecutor
	order                   []string // insertion order for stable listing
	cycleResolver           CycleResolver
	cycleUpdater            CycleUpdater
	decisionBoundaryChecker DecisionBoundaryChecker
	readFiles               map[string]bool // tracks files read in current session (for read-before-edit)
}

// MarkFileRead records that a file was read in this session.
func (r *Registry) MarkFileRead(path string) {
	if r.readFiles == nil {
		r.readFiles = make(map[string]bool)
	}
	r.readFiles[path] = true
}

// WasFileRead checks if a file was read before attempting to edit.
func (r *Registry) WasFileRead(path string) bool {
	return r.readFiles != nil && r.readFiles[path]
}

// SetCycleResolver sets the function that tools use to get the active cycle.
func (r *Registry) SetCycleResolver(resolver CycleResolver) {
	r.cycleResolver = resolver
}

// SetCycleUpdater sets the function that tools use to persist repaired cycles.
func (r *Registry) SetCycleUpdater(updater CycleUpdater) {
	r.cycleUpdater = updater
}

// ActiveCycle returns the current active cycle, or nil if none.
func (r *Registry) ActiveCycle(ctx context.Context) *agent.Cycle {
	if r.cycleResolver == nil {
		return nil
	}
	return r.cycleResolver(ctx)
}

// UpdateCycle persists cycle changes when an updater is available.
func (r *Registry) UpdateCycle(ctx context.Context, cycle *agent.Cycle) error {
	if r.cycleUpdater == nil || cycle == nil {
		return nil
	}
	return r.cycleUpdater(ctx, cycle)
}

// SetDecisionBoundaryChecker sets the function that checks whether the
// Transformer Mandate has been satisfied at the compare -> decide boundary.
func (r *Registry) SetDecisionBoundaryChecker(checker DecisionBoundaryChecker) {
	r.decisionBoundaryChecker = checker
}

// DecisionBoundarySatisfied returns true when a post-compare user selection is
// available. Returns true if no checker is set (backwards compatibility).
func (r *Registry) DecisionBoundarySatisfied(ctx context.Context, cycle *agent.Cycle) (bool, error) {
	if r.decisionBoundaryChecker == nil {
		return true, nil
	}
	return r.decisionBoundaryChecker(ctx, cycle)
}

// NewRegistry creates a registry with all builtin tools registered.
func NewRegistry(projectRoot string) *Registry {
	r := &Registry{tools: make(map[string]ToolExecutor)}
	r.register(&BashTool{projectRoot: projectRoot})
	r.register(&ReadFileTool{})
	r.register(&WriteFileTool{})
	r.register(&EditFileTool{registry: r})
	r.register(&MultiEditTool{registry: r})
	r.register(&GlobTool{projectRoot: projectRoot})
	r.register(&GrepTool{projectRoot: projectRoot})
	r.register(&FetchTool{})
	return r
}

func (r *Registry) register(t ToolExecutor) {
	r.tools[t.Name()] = t
	r.order = append(r.order, t.Name())
}

// Register adds a tool to the registry. Used to add haft kernel tools.
func (r *Registry) Register(t ToolExecutor) {
	r.register(t)
}

// List returns schemas for all registered tools (stable order).
func (r *Registry) List() []agent.ToolSchema {
	schemas := make([]agent.ToolSchema, 0, len(r.order))
	for _, name := range r.order {
		schemas = append(schemas, r.tools[name].Schema())
	}
	return schemas
}

// Get returns a tool executor by name.
func (r *Registry) Get(name string) (ToolExecutor, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// ReadOnlyRegistry returns a filtered registry excluding write tools.
// Used for read-only subagents (explore, plan).
func (r *Registry) ReadOnlyRegistry() *Registry {
	writeTools := map[string]bool{
		"bash": true, "write": true, "edit": true, "multiedit": true,
		"haft_problem": true, "haft_solution": true, "haft_decision": true, "haft_note": true,
	}
	ro := &Registry{tools: make(map[string]ToolExecutor)}
	for _, name := range r.order {
		if !writeTools[name] {
			ro.register(r.tools[name])
		}
	}
	return ro
}

// Execute runs a tool by name with the given arguments JSON.
func (r *Registry) Execute(ctx context.Context, name string, argsJSON string) (agent.ToolResult, error) {
	t, ok := r.tools[name]
	if !ok {
		return agent.ToolResult{}, fmt.Errorf("unknown tool: %s", name)
	}
	return t.Execute(ctx, argsJSON)
}
