package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/agent"
)

// WebSearchTool performs web searches using Brave Search API.
// Requires BRAVE_SEARCH_API_KEY environment variable.
type WebSearchTool struct {
	apiKey     string
	httpClient *http.Client
}

func NewWebSearchTool(apiKey string) *WebSearchTool {
	return &WebSearchTool{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "web_search",
		Description: `Search the web for information. Returns search results with titles, URLs, and snippets.

Requires BRAVE_SEARCH_API_KEY to be configured.

Use when you need:
- Current documentation or API references
- Error message explanations
- Recent changes to libraries or frameworks
- Information not available in the codebase`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return (default: 5, max: 10)",
				},
			},
			"required": []string{"query"},
		},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
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
	if args.MaxResults > 10 {
		args.MaxResults = 10
	}

	if t.apiKey == "" {
		return agent.PlainResult("Web search not configured. Set BRAVE_SEARCH_API_KEY in your environment or via `haft setup`."), nil
	}

	u := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(args.Query), args.MaxResults)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return agent.ToolResult{}, fmt.Errorf("Brave API returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 100_000))
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("read response: %w", err)
	}

	var result braveSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse response: %w", err)
	}

	if len(result.Web.Results) == 0 {
		return agent.PlainResult("No results found."), nil
	}

	var b strings.Builder
	for i, r := range result.Web.Results {
		if i >= args.MaxResults {
			break
		}
		fmt.Fprintf(&b, "%d. **%s**\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Description)
	}
	return agent.PlainResult(strings.TrimSpace(b.String())), nil
}

type braveSearchResponse struct {
	Web struct {
		Results []braveWebResult `json:"results"`
	} `json:"web"`
}

type braveWebResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}
