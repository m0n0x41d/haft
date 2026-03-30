package lsp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/m0n0x41d/haft/logger"
)

// ---------------------------------------------------------------------------
// L3: Manager — server lifecycle, lazy startup, multi-server orchestration
// ---------------------------------------------------------------------------

// Manager owns all LSP server clients and handles lazy startup.
// Thread-safe. Clients are started on-demand when matching files are edited.
type Manager struct {
	projectRoot string
	configs     []ServerConfig
	clients     map[string]*Client // key: server name
	clientsMu   sync.RWMutex
	unavailable map[string]bool    // servers that failed to start — don't retry
	callback    func(name string, state ServerState, counts DiagnosticCounts)
}

// NewManager creates a manager with the given server configurations.
func NewManager(projectRoot string, configs []ServerConfig) *Manager {
	return &Manager{
		projectRoot: projectRoot,
		configs:     configs,
		clients:     make(map[string]*Client),
		unavailable: make(map[string]bool),
	}
}

// SetCallback registers a function called on server state or diagnostic changes.
func (m *Manager) SetCallback(fn func(string, ServerState, DiagnosticCounts)) {
	m.callback = fn
}

// EnsureForFile starts any LSP servers that handle the given file.
// Lazy: only starts if not already running. Non-blocking for already-started servers.
func (m *Manager) EnsureForFile(ctx context.Context, path string) {
	for _, cfg := range m.configs {
		if m.unavailable[cfg.Name] {
			continue
		}

		m.clientsMu.RLock()
		_, exists := m.clients[cfg.Name]
		m.clientsMu.RUnlock()
		if exists {
			continue
		}

		if !matchesFileType(path, cfg.FileTypes) {
			continue
		}
		if !hasRootMarkers(m.projectRoot, cfg.RootMarkers) {
			continue
		}

		// Check if command is available
		if _, err := exec.LookPath(cfg.Command); err != nil {
			m.unavailable[cfg.Name] = true
			logger.Debug().Str("component", "lsp").
				Str("server", cfg.Name).
				Str("command", cfg.Command).
				Msg("lsp.command_not_found")
			continue
		}

		// Start in background
		go m.startServer(ctx, cfg)
	}
}

func (m *Manager) startServer(ctx context.Context, cfg ServerConfig) {
	client := NewClient(cfg, m.projectRoot)
	client.SetDiagnosticsCallback(func(name string, counts DiagnosticCounts) {
		if m.callback != nil {
			m.callback(name, StateReady, counts)
		}
	})

	if err := client.Start(ctx); err != nil {
		logger.Warn().Str("component", "lsp").
			Str("server", cfg.Name).
			Err(err).
			Msg("lsp.start_failed")
		m.unavailable[cfg.Name] = true
		if m.callback != nil {
			m.callback(cfg.Name, StateError, DiagnosticCounts{})
		}
		return
	}

	m.clientsMu.Lock()
	m.clients[cfg.Name] = client
	m.clientsMu.Unlock()

	if m.callback != nil {
		m.callback(cfg.Name, StateReady, DiagnosticCounts{})
	}
}

// NotifyFileChanged opens the file in matching servers and sends didChange.
// Call after tool edits. Returns after all servers have been notified.
func (m *Manager) NotifyFileChanged(ctx context.Context, path string) {
	m.EnsureForFile(ctx, path)

	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()

	for _, client := range m.clients {
		if !client.HandlesFile(path) {
			continue
		}
		_ = client.OpenFile(ctx, path)
		_ = client.NotifyChange(ctx, path)
	}
}

// WaitForDiagnostics blocks until diagnostics settle across all servers.
func (m *Manager) WaitForDiagnostics(ctx context.Context, path string) {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()

	for _, client := range m.clients {
		if !client.HandlesFile(path) {
			continue
		}
		client.WaitForDiagnostics(ctx, 5*secondDuration)
	}
}

const secondDuration = 1_000_000_000 // time.Second in nanoseconds, avoids import

// GetDiagnostics returns diagnostics from all servers, optionally filtered by file.
func (m *Manager) GetDiagnostics(file string) []Diagnostic {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()

	var all []Diagnostic
	for _, client := range m.clients {
		all = append(all, client.GetDiagnostics(file)...)
	}
	return all
}

// GetDiagnosticCounts returns aggregated counts from all servers.
func (m *Manager) GetDiagnosticCounts() DiagnosticCounts {
	return CountDiagnostics(m.GetDiagnostics(""))
}

// FindReferences queries the first matching server for references.
func (m *Manager) FindReferences(ctx context.Context, path string, line, col int) ([]Location, error) {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()

	for _, client := range m.clients {
		if !client.HandlesFile(path) {
			continue
		}
		return client.FindReferences(ctx, path, line, col)
	}
	return nil, nil
}

// RestartServer restarts a specific server by name, or all if name is empty.
func (m *Manager) RestartServer(ctx context.Context, name string) error {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	if name != "" {
		client, ok := m.clients[name]
		if !ok {
			return nil
		}
		client.Stop(ctx)
		delete(m.clients, name)
		delete(m.unavailable, name)
		// Will restart lazily on next file edit
		return nil
	}

	// Restart all
	for n, client := range m.clients {
		client.Stop(ctx)
		delete(m.clients, n)
	}
	m.unavailable = make(map[string]bool)
	return nil
}

// StopAll stops all running LSP servers.
func (m *Manager) StopAll(ctx context.Context) {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	for _, client := range m.clients {
		client.Stop(ctx)
	}
	m.clients = make(map[string]*Client)
}

// ServerStates returns current state of all configured servers.
func (m *Manager) ServerStates() map[string]ServerState {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()

	states := make(map[string]ServerState, len(m.configs))
	for _, cfg := range m.configs {
		if m.unavailable[cfg.Name] {
			states[cfg.Name] = StateError
		} else if client, ok := m.clients[cfg.Name]; ok {
			states[cfg.Name] = client.State()
		} else {
			states[cfg.Name] = StateUnstarted
		}
	}
	return states
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func matchesFileType(path string, fileTypes []string) bool {
	ext := filepath.Ext(path)
	for _, ft := range fileTypes {
		if ft == ext {
			return true
		}
	}
	return false
}

func hasRootMarkers(root string, markers []string) bool {
	if len(markers) == 0 {
		return true // no markers required
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(root, marker)); err == nil {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Default server configurations — known language servers
// ---------------------------------------------------------------------------

// DefaultConfigs returns server configs for common languages.
// Only servers that are installed (on PATH) will actually start.
// User configs (from config file) are merged on top via MergeConfigs.
func DefaultConfigs() []ServerConfig {
	return []ServerConfig{
		// Go
		{
			Name:        "gopls",
			Command:     "gopls",
			Args:        []string{"serve"},
			FileTypes:   []string{".go"},
			RootMarkers: []string{"go.mod", "go.sum"},
		},
		// TypeScript / JavaScript
		{
			Name:        "typescript-language-server",
			Command:     "typescript-language-server",
			Args:        []string{"--stdio"},
			FileTypes:   []string{".ts", ".tsx", ".js", ".jsx"},
			RootMarkers: []string{"package.json", "tsconfig.json"},
		},
		// Python
		{
			Name:        "pyright",
			Command:     "pyright-langserver",
			Args:        []string{"--stdio"},
			FileTypes:   []string{".py"},
			RootMarkers: []string{"pyproject.toml", "setup.py", "requirements.txt"},
		},
		// Rust
		{
			Name:        "rust-analyzer",
			Command:     "rust-analyzer",
			FileTypes:   []string{".rs"},
			RootMarkers: []string{"Cargo.toml"},
		},
		// C / C++
		{
			Name:        "clangd",
			Command:     "clangd",
			FileTypes:   []string{".c", ".cpp", ".cc", ".h", ".hpp"},
			RootMarkers: []string{"compile_commands.json", "CMakeLists.txt", "Makefile"},
		},
		// Ruby
		{
			Name:        "solargraph",
			Command:     "solargraph",
			Args:        []string{"stdio"},
			FileTypes:   []string{".rb"},
			RootMarkers: []string{"Gemfile"},
		},
		// Java
		{
			Name:        "jdtls",
			Command:     "jdtls",
			FileTypes:   []string{".java"},
			RootMarkers: []string{"pom.xml", "build.gradle", "build.gradle.kts"},
		},
		// Kotlin
		{
			Name:        "kotlin-language-server",
			Command:     "kotlin-language-server",
			FileTypes:   []string{".kt", ".kts"},
			RootMarkers: []string{"build.gradle", "build.gradle.kts"},
		},
		// PHP
		{
			Name:        "intelephense",
			Command:     "intelephense",
			Args:        []string{"--stdio"},
			FileTypes:   []string{".php"},
			RootMarkers: []string{"composer.json"},
		},
		// Lua
		{
			Name:        "lua-language-server",
			Command:     "lua-language-server",
			FileTypes:   []string{".lua"},
			RootMarkers: []string{".luarc.json", ".luarc.jsonc"},
		},
		// Zig
		{
			Name:        "zls",
			Command:     "zls",
			FileTypes:   []string{".zig"},
			RootMarkers: []string{"build.zig"},
		},
		// Swift
		{
			Name:        "sourcekit-lsp",
			Command:     "sourcekit-lsp",
			FileTypes:   []string{".swift"},
			RootMarkers: []string{"Package.swift"},
		},
		// C#
		{
			Name:    "omnisharp",
			Command: "OmniSharp",
			Args:    []string{"--languageserver"},
			FileTypes:   []string{".cs"},
			RootMarkers: []string{"*.sln", "*.csproj"},
		},
		// Elixir
		{
			Name:        "elixir-ls",
			Command:     "elixir-ls",
			FileTypes:   []string{".ex", ".exs"},
			RootMarkers: []string{"mix.exs"},
		},
		// Haskell
		{
			Name:        "haskell-language-server",
			Command:     "haskell-language-server-wrapper",
			Args:        []string{"--lsp"},
			FileTypes:   []string{".hs"},
			RootMarkers: []string{"stack.yaml", "cabal.project", "*.cabal"},
		},
		// Scala
		{
			Name:        "metals",
			Command:     "metals",
			FileTypes:   []string{".scala", ".sc"},
			RootMarkers: []string{"build.sbt", "build.sc"},
		},
		// Dart
		{
			Name:        "dart",
			Command:     "dart",
			Args:        []string{"language-server", "--protocol=lsp"},
			FileTypes:   []string{".dart"},
			RootMarkers: []string{"pubspec.yaml"},
		},
		// Vue
		{
			Name:        "vue-language-server",
			Command:     "vue-language-server",
			Args:        []string{"--stdio"},
			FileTypes:   []string{".vue"},
			RootMarkers: []string{"package.json"},
		},
		// Svelte
		{
			Name:        "svelte-language-server",
			Command:     "svelteserver",
			Args:        []string{"--stdio"},
			FileTypes:   []string{".svelte"},
			RootMarkers: []string{"package.json"},
		},
		// Bash / Shell
		{
			Name:        "bash-language-server",
			Command:     "bash-language-server",
			Args:        []string{"start"},
			FileTypes:   []string{".sh", ".bash", ".zsh"},
			RootMarkers: []string{},
		},
		// YAML
		{
			Name:        "yaml-language-server",
			Command:     "yaml-language-server",
			Args:        []string{"--stdio"},
			FileTypes:   []string{".yaml", ".yml"},
			RootMarkers: []string{},
		},
		// Terraform
		{
			Name:        "terraform-ls",
			Command:     "terraform-ls",
			Args:        []string{"serve"},
			FileTypes:   []string{".tf", ".tfvars"},
			RootMarkers: []string{".terraform", "main.tf"},
		},
		// Dockerfile
		{
			Name:        "docker-langserver",
			Command:     "docker-langserver",
			Args:        []string{"--stdio"},
			FileTypes:   []string{".dockerfile"},
			RootMarkers: []string{"Dockerfile", "docker-compose.yml"},
		},
		// OCaml
		{
			Name:        "ocaml-lsp",
			Command:     "ocamllsp",
			FileTypes:   []string{".ml", ".mli"},
			RootMarkers: []string{"dune-project", "dune"},
		},
		// Clojure
		{
			Name:        "clojure-lsp",
			Command:     "clojure-lsp",
			FileTypes:   []string{".clj", ".cljs", ".cljc", ".edn"},
			RootMarkers: []string{"deps.edn", "project.clj"},
		},
		// Nix
		{
			Name:        "nixd",
			Command:     "nixd",
			FileTypes:   []string{".nix"},
			RootMarkers: []string{"flake.nix", "default.nix"},
		},
		// Gleam
		{
			Name:        "gleam",
			Command:     "gleam",
			Args:        []string{"lsp"},
			FileTypes:   []string{".gleam"},
			RootMarkers: []string{"gleam.toml"},
		},
		// Deno (separate from tsserver — root marker distinguishes)
		{
			Name:        "deno",
			Command:     "deno",
			Args:        []string{"lsp"},
			FileTypes:   []string{".ts", ".tsx", ".js", ".jsx"},
			RootMarkers: []string{"deno.json", "deno.jsonc"},
		},
		// F#
		{
			Name:        "fsautocomplete",
			Command:     "fsautocomplete",
			Args:        []string{"--adaptive-lsp-server-enabled"},
			FileTypes:   []string{".fs", ".fsi", ".fsx"},
			RootMarkers: []string{"*.fsproj", "*.sln"},
		},
		// Prisma
		{
			Name:        "prisma-language-server",
			Command:     "prisma-language-server",
			Args:        []string{"--stdio"},
			FileTypes:   []string{".prisma"},
			RootMarkers: []string{"prisma/schema.prisma"},
		},
		// Astro
		{
			Name:        "astro-ls",
			Command:     "astro-ls",
			Args:        []string{"--stdio"},
			FileTypes:   []string{".astro"},
			RootMarkers: []string{"package.json"},
		},
		// Julia
		{
			Name:        "julia-lsp",
			Command:     "julia",
			Args:        []string{"--startup-file=no", "-e", "using LanguageServer; runserver()"},
			FileTypes:   []string{".jl"},
			RootMarkers: []string{"Project.toml"},
		},
		// LaTeX
		{
			Name:        "texlab",
			Command:     "texlab",
			FileTypes:   []string{".tex", ".bib"},
			RootMarkers: []string{},
		},
		// Typst
		{
			Name:        "tinymist",
			Command:     "tinymist",
			FileTypes:   []string{".typ"},
			RootMarkers: []string{},
		},
		// R
		{
			Name:        "r-languageserver",
			Command:     "R",
			Args:        []string{"--slave", "-e", "languageserver::run()"},
			FileTypes:   []string{".r", ".R"},
			RootMarkers: []string{"DESCRIPTION", ".Rproj"},
		},
		// Erlang
		{
			Name:        "erlang-ls",
			Command:     "erlang_ls",
			FileTypes:   []string{".erl", ".hrl"},
			RootMarkers: []string{"rebar.config", "erlang.mk"},
		},
	}
}

// MergeConfigs combines user configs with defaults.
// User configs override defaults by name; new names are appended.
// Pure function.
func MergeConfigs(defaults, user []ServerConfig) []ServerConfig {
	byName := make(map[string]ServerConfig, len(defaults))
	order := make([]string, 0, len(defaults))
	for _, cfg := range defaults {
		byName[cfg.Name] = cfg
		order = append(order, cfg.Name)
	}
	for _, cfg := range user {
		if _, exists := byName[cfg.Name]; !exists {
			order = append(order, cfg.Name)
		}
		byName[cfg.Name] = cfg // user wins
	}
	result := make([]ServerConfig, 0, len(order))
	for _, name := range order {
		result = append(result, byName[name])
	}
	return result
}
