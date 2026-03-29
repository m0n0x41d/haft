package tools

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

// ToolExecutor is the interface each tool implements.
type ToolExecutor interface {
	Name() string
	Schema() agent.ToolSchema
	Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error)
}

// Registry holds all available tools.
type Registry struct {
	tools map[string]ToolExecutor
	order []string // insertion order for stable listing
}

// NewRegistry creates a registry with all builtin tools registered.
func NewRegistry(projectRoot string) *Registry {
	r := &Registry{tools: make(map[string]ToolExecutor)}
	r.register(&BashTool{projectRoot: projectRoot})
	r.register(&ReadFileTool{})
	r.register(&WriteFileTool{})
	r.register(&EditFileTool{})
	r.register(&GlobTool{projectRoot: projectRoot})
	r.register(&GrepTool{projectRoot: projectRoot})
	return r
}

func (r *Registry) register(t ToolExecutor) {
	r.tools[t.Name()] = t
	r.order = append(r.order, t.Name())
}

// Register adds a tool to the registry. Used to add quint kernel tools.
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

// Execute runs a tool by name with the given arguments JSON.
func (r *Registry) Execute(ctx context.Context, name string, argsJSON string) (agent.ToolResult, error) {
	t, ok := r.tools[name]
	if !ok {
		return agent.ToolResult{}, fmt.Errorf("unknown tool: %s", name)
	}
	return t.Execute(ctx, argsJSON)
}
