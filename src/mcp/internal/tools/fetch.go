package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	htmltomd "github.com/JohannesKaufmann/html-to-markdown/v2"
	// v2 uses package-level ConvertString
	"github.com/m0n0x41d/quint-code/internal/agent"
	"golang.org/x/net/html"
)

// FetchTool retrieves URL content and converts HTML to readable markdown.
// No headless browser — pure HTTP GET + HTML-to-markdown conversion.
// Handles: docs, blog posts, GitHub READMEs, API pages, code samples.
type FetchTool struct{}

const (
	fetchMaxBytes  = 1024 * 1024 // 1 MB
	fetchTimeout   = 30 * time.Second
	fetchUserAgent = "Mozilla/5.0 (compatible; HaftAgent/1.0)"
	fetchMaxOutput = 50000 // chars — truncate if longer
)

type fetchArgs struct {
	URL string `json:"url"`
}

func (t *FetchTool) Name() string { return "fetch" }

func (t *FetchTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "fetch",
		Description: "Fetch a URL and return its content as markdown. Converts HTML pages to readable text. Works for documentation, blog posts, API docs, GitHub pages. Returns raw text for non-HTML content (JSON, plain text).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "URL to fetch",
				},
			},
			"required": []any{"url"},
		},
	}
}

func (t *FetchTool) Execute(_ context.Context, argsJSON string) (agent.ToolResult, error) {
	var args fetchArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if args.URL == "" {
		return agent.ToolResult{}, fmt.Errorf("url is required")
	}

	// HTTP GET with timeout and size limit
	client := &http.Client{Timeout: fetchTimeout}
	req, err := http.NewRequest("GET", args.URL, nil)
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("invalid url: %w", err)
	}
	req.Header.Set("User-Agent", fetchUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return agent.PlainResult(fmt.Sprintf("Fetch failed: %s", err.Error())), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return agent.PlainResult(fmt.Sprintf("HTTP %d %s", resp.StatusCode, resp.Status)), nil
	}

	// Read with size limit
	body, err := io.ReadAll(io.LimitReader(resp.Body, fetchMaxBytes))
	if err != nil {
		return agent.PlainResult(fmt.Sprintf("Read error: %s", err.Error())), nil
	}

	contentType := resp.Header.Get("Content-Type")
	content := string(body)

	// Non-HTML content — return as-is
	if !strings.Contains(contentType, "html") {
		if len([]rune(content)) > fetchMaxOutput {
			content = string([]rune(content)[:fetchMaxOutput]) + "\n... (truncated)"
		}
		return agent.PlainResult(content), nil
	}

	// HTML → clean → markdown
	markdown, err := htmlToMarkdown(content)
	if err != nil {
		// Fallback: strip tags
		markdown = stripHTMLTags(content)
	}

	// Truncate if too long
	if len([]rune(markdown)) > fetchMaxOutput {
		markdown = string([]rune(markdown)[:fetchMaxOutput]) + "\n... (truncated)"
	}

	return agent.PlainResult(markdown), nil
}

// htmlToMarkdown converts HTML to readable markdown.
// Removes noisy elements (nav, footer, script, style) before conversion.
func htmlToMarkdown(rawHTML string) (string, error) {
	// Remove noisy elements before conversion
	cleaned := removeNoisyElements(rawHTML)

	// Convert HTML → Markdown (v2 package-level function)
	md, err := htmltomd.ConvertString(cleaned)
	if err != nil {
		return "", err
	}

	// Cleanup: collapse excessive newlines
	for strings.Contains(md, "\n\n\n") {
		md = strings.ReplaceAll(md, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(md), nil
}

// removeNoisyElements strips script, style, nav, header, footer, aside, iframe from HTML.
func removeNoisyElements(rawHTML string) string {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return rawHTML
	}

	removeElements(doc, map[string]bool{
		"script": true, "style": true, "nav": true, "header": true,
		"footer": true, "aside": true, "noscript": true, "iframe": true, "svg": true,
	})

	var b strings.Builder
	if err := html.Render(&b, doc); err != nil {
		return rawHTML
	}
	return b.String()
}

// removeElements recursively removes elements with specified tag names.
func removeElements(n *html.Node, tags map[string]bool) {
	var next *html.Node
	for c := n.FirstChild; c != nil; c = next {
		next = c.NextSibling
		if c.Type == html.ElementNode && tags[c.Data] {
			n.RemoveChild(c)
		} else {
			removeElements(c, tags)
		}
	}
}

// stripHTMLTags is a simple fallback — removes all HTML tags.
func stripHTMLTags(s string) string {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return s
	}
	var b strings.Builder
	extractText(doc, &b)
	return strings.TrimSpace(b.String())
}

func extractText(n *html.Node, b *strings.Builder) {
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			b.WriteString(text)
			b.WriteString(" ")
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, b)
	}
}
