package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/agent"
)

// ToolSearchTool allows the LLM to discover and load tools that aren't in the
// initial schema. This reduces context usage — only core tools are sent upfront,
// and specialized tools are loaded on demand.
//
// When the LLM calls tool_search, it gets back the full schema for matching tools,
// which it can then call in subsequent turns.
type ToolSearchTool struct {
	registry *Registry
	deferred map[string]agent.ToolSchema // tools not in the initial set
}

func NewToolSearchTool(registry *Registry, deferred map[string]agent.ToolSchema) *ToolSearchTool {
	return &ToolSearchTool{
		registry: registry,
		deferred: deferred,
	}
}

func (t *ToolSearchTool) Name() string { return "tool_search" }

func (t *ToolSearchTool) Schema() agent.ToolSchema {
	// Build the list of searchable tool names for the description
	names := make([]string, 0, len(t.deferred))
	for name := range t.deferred {
		names = append(names, name)
	}

	return agent.ToolSchema{
		Name: "tool_search",
		Description: fmt.Sprintf(`Search for and load additional tools not in the initial tool set. Returns full tool schemas that you can call in subsequent turns.

Available deferred tools: %s

Use "select:<name>" for exact match, or keywords to search by description.`, strings.Join(names, ", ")),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query. Use 'select:tool_name' for exact match, or keywords to search descriptions.",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "Maximum results to return (default: 5)",
				},
			},
			"required": []string{"query"},
		},
	}
}

func (t *ToolSearchTool) Execute(_ context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Query == "" {
		return agent.ToolResult{}, fmt.Errorf("query is required")
	}
	if args.MaxResults <= 0 {
		args.MaxResults = 5
	}

	// Exact match: "select:tool_name" or "select:name1,name2"
	if strings.HasPrefix(args.Query, "select:") {
		names := strings.Split(strings.TrimPrefix(args.Query, "select:"), ",")
		return t.selectByName(names)
	}

	// Keyword search across deferred tool names and descriptions
	return t.searchByKeyword(args.Query, args.MaxResults)
}

func (t *ToolSearchTool) selectByName(names []string) (agent.ToolResult, error) {
	var found []agent.ToolSchema
	var notFound []string

	for _, name := range names {
		name = strings.TrimSpace(name)
		if schema, ok := t.deferred[name]; ok {
			found = append(found, schema)
			// Register the tool so it becomes callable
			// (the tool executor must already be in the registry)
		} else {
			notFound = append(notFound, name)
		}
	}

	return agent.PlainResult(formatToolSearchResults(found, notFound)), nil
}

func (t *ToolSearchTool) searchByKeyword(query string, max int) (agent.ToolResult, error) {
	lower := strings.ToLower(query)
	var matches []agent.ToolSchema

	for name, schema := range t.deferred {
		if strings.Contains(strings.ToLower(name), lower) ||
			strings.Contains(strings.ToLower(schema.Description), lower) {
			matches = append(matches, schema)
			if len(matches) >= max {
				break
			}
		}
	}

	return agent.PlainResult(formatToolSearchResults(matches, nil)), nil
}

func formatToolSearchResults(found []agent.ToolSchema, notFound []string) string {
	if len(found) == 0 && len(notFound) == 0 {
		return "No matching tools found."
	}

	var b strings.Builder
	if len(found) > 0 {
		b.WriteString("## Found tools\n\n")
		for _, schema := range found {
			params, _ := json.MarshalIndent(schema.Parameters, "  ", "  ")
			fmt.Fprintf(&b, "### %s\n%s\n\nParameters:\n```json\n  %s\n```\n\n",
				schema.Name, schema.Description, string(params))
		}
	}

	if len(notFound) > 0 {
		fmt.Fprintf(&b, "Not found: %s\n", strings.Join(notFound, ", "))
	}

	return strings.TrimSpace(b.String())
}
