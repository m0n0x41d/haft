// Package provider — model registry (catwalk pattern).
//
// Provides a curated catalog of LLM providers and models with metadata
// (context window, max output tokens, reasoning support, image support, cost).
//
// Architecture:
//   L0: ModelInfo, ProviderInfo — pure data types
//   L1: Registry, Lookup, Merge — pure functions on data
//   L2: FetchRemote, ReadCache, WriteCache — thin I/O boundary
//   L3: LoadRegistry — orchestration with fallback chain
//
// Fallback chain: remote (catwalk.charm.sh) → disk cache → embedded defaults.
// Remote fetch is best-effort with ETag caching. Never blocks startup on network.
package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// L0: Pure data types
// ---------------------------------------------------------------------------

// ModelInfo describes a single LLM model's capabilities and constraints.
type ModelInfo struct {
	ID             string  `json:"id"`
	Name           string  `json:"name,omitempty"`
	ContextWindow  int     `json:"context_window"`
	DefaultMaxOut  int     `json:"default_max_tokens"`
	CanReason      bool    `json:"can_reason,omitempty"`
	SupportsImages bool    `json:"supports_images,omitempty"`
	CostPer1MIn    float64 `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut   float64 `json:"cost_per_1m_out,omitempty"`
}

// ProviderInfo describes an LLM provider and its available models.
type ProviderInfo struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	APIType string     `json:"type,omitempty"` // "openai", "anthropic", "google"
	Models []ModelInfo `json:"models"`
}

// ---------------------------------------------------------------------------
// L1: Registry — pure data structure with lookup
// ---------------------------------------------------------------------------

// Registry holds known providers and models. Thread-safe after construction.
type Registry struct {
	providers []ProviderInfo
	byModel   map[string]ModelInfo // key: model ID → flattened lookup
}

// NewRegistry builds a registry from a list of providers.
// Pure function — just indexes the data.
func NewRegistry(providers []ProviderInfo) *Registry {
	r := &Registry{
		providers: providers,
		byModel:   make(map[string]ModelInfo, 64),
	}
	for _, p := range providers {
		for _, m := range p.Models {
			r.byModel[m.ID] = m
		}
	}
	return r
}

// Lookup finds model info by exact ID, then by prefix match.
// Returns zero value and false if not found.
func (r *Registry) Lookup(modelID string) (ModelInfo, bool) {
	// Exact match first
	if m, ok := r.byModel[modelID]; ok {
		return m, true
	}
	// Prefix match: "gpt-5.4-turbo" matches "gpt-5.4" entry
	var best ModelInfo
	bestLen := 0
	for id, m := range r.byModel {
		if strings.HasPrefix(modelID, id) && len(id) > bestLen {
			best = m
			bestLen = len(id)
		}
	}
	if bestLen > 0 {
		return best, true
	}
	return ModelInfo{}, false
}

// ContextWindow returns the context window for a model, with a default fallback.
func (r *Registry) ContextWindow(modelID string) int {
	if m, ok := r.Lookup(modelID); ok && m.ContextWindow > 0 {
		return m.ContextWindow
	}
	return 128_000 // conservative default
}

// Providers returns all known providers.
func (r *Registry) Providers() []ProviderInfo {
	return r.providers
}

// MergeProviders combines remote + embedded provider lists.
// Remote adds new providers/models. For models that exist in both,
// keeps the one with the higher context window (embedded may have
// corrections that remote hasn't picked up yet).
// Pure function.
func MergeProviders(remote, embedded []ProviderInfo) []ProviderInfo {
	// Build embedded index: provider → model ID → ModelInfo
	embeddedModels := make(map[string]map[string]ModelInfo)
	for _, p := range embedded {
		m := make(map[string]ModelInfo, len(p.Models))
		for _, model := range p.Models {
			m[model.ID] = model
		}
		embeddedModels[p.ID] = m
	}

	// Start with remote, merge embedded corrections
	byID := make(map[string]ProviderInfo, len(remote)+len(embedded))
	for _, p := range remote {
		if emb, ok := embeddedModels[p.ID]; ok {
			// Merge models: keep higher context window
			for i, rm := range p.Models {
				if em, exists := emb[rm.ID]; exists && em.ContextWindow > rm.ContextWindow {
					p.Models[i].ContextWindow = em.ContextWindow
				}
			}
		}
		byID[p.ID] = p
	}
	// Add embedded-only providers (not in remote)
	for _, p := range embedded {
		if _, exists := byID[p.ID]; !exists {
			byID[p.ID] = p
		}
	}

	result := make([]ProviderInfo, 0, len(byID))
	for _, p := range byID {
		result = append(result, p)
	}
	return result
}

// ---------------------------------------------------------------------------
// L2: I/O boundary — fetch, cache, embed
// ---------------------------------------------------------------------------

const (
	defaultCatwalkURL = "https://catwalk.charm.sh/v2/providers"
	fetchTimeout      = 10 * time.Second
	cacheFileName     = "providers.json"
)

// FetchRemote fetches provider data from the catwalk API.
// Returns providers and a new ETag (for cache validation).
// Returns nil, "" on any error — caller falls back to cache/embedded.
func FetchRemote(ctx context.Context, url, etag string) ([]ProviderInfo, string) {
	if url == "" {
		url = defaultCatwalkURL
	}

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, ""
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, etag // cache still valid
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024)) // 2MB max
	if err != nil {
		return nil, ""
	}

	// Parse catwalk response — array of providers with nested models
	var raw []catwalkProvider
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, ""
	}

	providers := convertCatwalkResponse(raw)
	newETag := resp.Header.Get("ETag")
	if newETag == "" {
		// Generate our own ETag from content hash
		h := sha256.Sum256(body)
		newETag = hex.EncodeToString(h[:8])
	}

	return providers, newETag
}

// catwalkProvider mirrors the catwalk.charm.sh JSON response format.
type catwalkProvider struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Models []catwalkModel `json:"models"`
}

type catwalkModel struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	ContextWindow     int     `json:"context_window"`
	DefaultMaxTokens  int     `json:"default_max_tokens"`
	CanReason         bool    `json:"can_reason"`
	SupportsImages    bool    `json:"supports_images"`
	CostPer1MIn       float64 `json:"cost_per_1m_in"`
	CostPer1MOut      float64 `json:"cost_per_1m_out"`
}

func convertCatwalkResponse(raw []catwalkProvider) []ProviderInfo {
	providers := make([]ProviderInfo, 0, len(raw))
	for _, rp := range raw {
		p := ProviderInfo{
			ID:      rp.ID,
			Name:    rp.Name,
			APIType: rp.ID, // catwalk uses provider ID as API type
		}
		for _, rm := range rp.Models {
			p.Models = append(p.Models, ModelInfo{
				ID:             rm.ID,
				Name:           rm.Name,
				ContextWindow:  rm.ContextWindow,
				DefaultMaxOut:  rm.DefaultMaxTokens,
				CanReason:      rm.CanReason,
				SupportsImages: rm.SupportsImages,
				CostPer1MIn:    rm.CostPer1MIn,
				CostPer1MOut:   rm.CostPer1MOut,
			})
		}
		providers = append(providers, p)
	}
	return providers
}

// cacheEntry wraps cached data with ETag for conditional requests.
type cacheEntry struct {
	ETag      string         `json:"etag"`
	Providers []ProviderInfo `json:"providers"`
}

// ReadCache loads cached providers from disk.
func ReadCache(cacheDir string) ([]ProviderInfo, string) {
	path := filepath.Join(cacheDir, cacheFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ""
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, ""
	}
	return entry.Providers, entry.ETag
}

// WriteCache saves providers to disk for next startup.
func WriteCache(cacheDir string, providers []ProviderInfo, etag string) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return
	}
	entry := cacheEntry{ETag: etag, Providers: providers}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	path := filepath.Join(cacheDir, cacheFileName)
	_ = os.WriteFile(path, data, 0644)
}

// ---------------------------------------------------------------------------
// L3: LoadRegistry — orchestration with fallback chain
// ---------------------------------------------------------------------------

var (
	globalRegistry     *Registry
	globalRegistryOnce sync.Once
)

// LoadRegistry builds the model registry using the catwalk fallback chain:
//   1. Fetch remote (catwalk.charm.sh) with ETag — best-effort, 10s timeout
//   2. On 304 Not Modified or fetch error → use disk cache
//   3. Merge remote/cached with embedded defaults (embedded fills gaps)
//   4. Cache result for next startup
//
// Thread-safe, loads exactly once per process.
func LoadRegistry(cacheDir string) *Registry {
	globalRegistryOnce.Do(func() {
		embedded := EmbeddedProviders()

		// Try cache first to get ETag
		cached, etag := ReadCache(cacheDir)

		// Try remote fetch (non-blocking, 10s timeout)
		remote, newETag := FetchRemote(context.Background(), "", etag)

		var providers []ProviderInfo
		switch {
		case len(remote) > 0:
			// Fresh data from remote — merge with embedded
			providers = MergeProviders(remote, embedded)
			WriteCache(cacheDir, providers, newETag)
		case len(cached) > 0:
			// Remote failed or not modified — use cache merged with embedded
			providers = MergeProviders(cached, embedded)
		default:
			// No remote, no cache — embedded only
			providers = embedded
		}

		globalRegistry = NewRegistry(providers)
	})
	return globalRegistry
}

// DefaultRegistry returns the global registry, loading it if needed.
// Uses ~/.haft/ as cache directory.
func DefaultRegistry() *Registry {
	home, err := os.UserHomeDir()
	if err != nil {
		return NewRegistry(EmbeddedProviders())
	}
	return LoadRegistry(filepath.Join(home, ".haft", "cache"))
}

// ---------------------------------------------------------------------------
// Embedded defaults — known models compiled into the binary
// ---------------------------------------------------------------------------

// EmbeddedProviders returns the built-in model catalog.
// This is the fallback when remote fetch and cache both fail.
// Updated periodically with releases. Pure function.
func EmbeddedProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			ID: "openai", Name: "OpenAI", APIType: "openai",
			Models: []ModelInfo{
				{ID: "gpt-5.4", Name: "GPT-5.4", ContextWindow: 1_050_000, DefaultMaxOut: 16384, CanReason: true, SupportsImages: true, CostPer1MIn: 2.0, CostPer1MOut: 8.0},
				{ID: "gpt-5.3", Name: "GPT-5.3", ContextWindow: 400_000, DefaultMaxOut: 16384, CanReason: true, SupportsImages: true, CostPer1MIn: 2.5, CostPer1MOut: 10.0},
				{ID: "gpt-5.2", Name: "GPT-5.2", ContextWindow: 256_000, DefaultMaxOut: 16384, CanReason: true, SupportsImages: true, CostPer1MIn: 3.0, CostPer1MOut: 12.0},
				{ID: "gpt-5.1", Name: "GPT-5.1", ContextWindow: 256_000, DefaultMaxOut: 16384, CanReason: true, SupportsImages: true, CostPer1MIn: 3.0, CostPer1MOut: 12.0},
				{ID: "gpt-5-mini", Name: "GPT-5 Mini", ContextWindow: 1_050_000, DefaultMaxOut: 16384, CanReason: true, SupportsImages: true, CostPer1MIn: 0.3, CostPer1MOut: 1.2},
				{ID: "gpt-4o", Name: "GPT-4o", ContextWindow: 128_000, DefaultMaxOut: 4096, CanReason: false, SupportsImages: true, CostPer1MIn: 2.5, CostPer1MOut: 10.0},
				{ID: "gpt-4o-mini", Name: "GPT-4o Mini", ContextWindow: 128_000, DefaultMaxOut: 4096, CanReason: false, SupportsImages: true, CostPer1MIn: 0.15, CostPer1MOut: 0.6},
				{ID: "gpt-4.1", Name: "GPT-4.1", ContextWindow: 128_000, DefaultMaxOut: 16384, CanReason: false, SupportsImages: true, CostPer1MIn: 2.0, CostPer1MOut: 8.0},
				{ID: "gpt-4.1-mini", Name: "GPT-4.1 Mini", ContextWindow: 128_000, DefaultMaxOut: 16384, CanReason: false, SupportsImages: true, CostPer1MIn: 0.4, CostPer1MOut: 1.6},
				{ID: "o4-mini", Name: "o4-mini", ContextWindow: 200_000, DefaultMaxOut: 100_000, CanReason: true, SupportsImages: true, CostPer1MIn: 1.1, CostPer1MOut: 4.4},
				{ID: "o3", Name: "o3", ContextWindow: 200_000, DefaultMaxOut: 100_000, CanReason: true, SupportsImages: true, CostPer1MIn: 2.0, CostPer1MOut: 8.0},
				{ID: "o3-mini", Name: "o3-mini", ContextWindow: 200_000, DefaultMaxOut: 65_536, CanReason: true, SupportsImages: false, CostPer1MIn: 1.1, CostPer1MOut: 4.4},
				{ID: "o1", Name: "o1", ContextWindow: 200_000, DefaultMaxOut: 32_768, CanReason: true, SupportsImages: true, CostPer1MIn: 15.0, CostPer1MOut: 60.0},
			},
		},
		{
			ID: "anthropic", Name: "Anthropic", APIType: "anthropic",
			Models: []ModelInfo{
				{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", ContextWindow: 200_000, DefaultMaxOut: 32_000, CanReason: true, SupportsImages: true, CostPer1MIn: 15.0, CostPer1MOut: 75.0},
				{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", ContextWindow: 200_000, DefaultMaxOut: 16_000, CanReason: true, SupportsImages: true, CostPer1MIn: 3.0, CostPer1MOut: 15.0},
				{ID: "claude-haiku-3-5-20241022", Name: "Claude Haiku 3.5", ContextWindow: 200_000, DefaultMaxOut: 8_192, CanReason: false, SupportsImages: true, CostPer1MIn: 0.8, CostPer1MOut: 4.0},
			},
		},
		{
			ID: "google", Name: "Google", APIType: "google",
			Models: []ModelInfo{
				{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", ContextWindow: 1_000_000, DefaultMaxOut: 65_536, CanReason: true, SupportsImages: true, CostPer1MIn: 1.25, CostPer1MOut: 10.0},
				{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", ContextWindow: 1_000_000, DefaultMaxOut: 65_536, CanReason: true, SupportsImages: true, CostPer1MIn: 0.15, CostPer1MOut: 0.6},
				{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", ContextWindow: 1_000_000, DefaultMaxOut: 8_192, CanReason: false, SupportsImages: true, CostPer1MIn: 0.1, CostPer1MOut: 0.4},
			},
		},
		{
			ID: "deepseek", Name: "DeepSeek", APIType: "openai",
			Models: []ModelInfo{
				{ID: "deepseek-chat", Name: "DeepSeek V3", ContextWindow: 128_000, DefaultMaxOut: 8_192, CanReason: false, SupportsImages: false, CostPer1MIn: 0.27, CostPer1MOut: 1.1},
				{ID: "deepseek-reasoner", Name: "DeepSeek R1", ContextWindow: 128_000, DefaultMaxOut: 8_192, CanReason: true, SupportsImages: false, CostPer1MIn: 0.55, CostPer1MOut: 2.19},
			},
		},
		{
			ID: "groq", Name: "Groq", APIType: "openai",
			Models: []ModelInfo{
				{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", ContextWindow: 128_000, DefaultMaxOut: 32_768, CanReason: false, SupportsImages: false, CostPer1MIn: 0.59, CostPer1MOut: 0.79},
			},
		},
	}
}

// FormatModelList renders the registry as a human-readable model list.
// Used by `haft models` command.
func (r *Registry) FormatModelList(filter string) string {
	var b strings.Builder
	for _, p := range r.providers {
		var matched []ModelInfo
		for _, m := range p.Models {
			if filter == "" || strings.Contains(strings.ToLower(m.ID), strings.ToLower(filter)) || strings.Contains(strings.ToLower(m.Name), strings.ToLower(filter)) {
				matched = append(matched, m)
			}
		}
		if len(matched) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s\n", p.Name)
		for _, m := range matched {
			ctx := formatTokenCount(m.ContextWindow)
			reason := ""
			if m.CanReason {
				reason = " [reason]"
			}
			fmt.Fprintf(&b, "  %-30s %6s ctx  $%.2f/$%.2f%s\n",
				m.ID, ctx, m.CostPer1MIn, m.CostPer1MOut, reason)
		}
	}
	return b.String()
}

func formatTokenCount(tokens int) string {
	switch {
	case tokens >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	case tokens >= 1_000:
		return fmt.Sprintf("%dk", tokens/1_000)
	default:
		return fmt.Sprintf("%d", tokens)
	}
}
