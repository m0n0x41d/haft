package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

var (
	initClaude bool
	initCursor bool
	initGemini bool
	initCodex  bool
	initAir    bool
	initAll    bool
	initLocal  bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize FPF project structure and MCP configuration",
	Long: `Initialize a new Haft project in the current directory.

This command creates:
  - .haft/ directory structure (knowledge base, evidence, decisions)
  - MCP configuration for selected AI tools
  - Slash commands / prompts (global by default, or local with --local)
  - Repo-local Air skills when requested

Examples:
  haft init              # Claude, global commands (~/.claude/commands/)
  haft init --local      # Claude, local commands (.claude/commands/)
  haft init --all        # All tools, global commands
  haft init --cursor     # Cursor only
  haft init --air        # Air skill + Codex-compatible prompts/MCP`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initClaude, "claude", false, "Configure for Claude Code")
	initCmd.Flags().BoolVar(&initCursor, "cursor", false, "Configure for Cursor")
	initCmd.Flags().BoolVar(&initGemini, "gemini", false, "Configure for Gemini CLI")
	initCmd.Flags().BoolVar(&initCodex, "codex", false, "Configure for Codex CLI")
	initCmd.Flags().BoolVar(&initAir, "air", false, "Configure for JetBrains Air")
	initCmd.Flags().BoolVar(&initAll, "all", false, "Configure for all supported tools")
	initCmd.Flags().BoolVar(&initLocal, "local", false, "Install commands in project directory instead of global")

	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Ensure global ~/.haft/ exists (migrates from ~/.quint-code/ if needed)
	_ = project.EnsureDir()

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	haftDir := filepath.Join(cwd, ".haft")
	oldQuintDir := filepath.Join(cwd, ".quint")

	// Migration: rename .quint/ → .haft/ if old directory exists
	if _, err := os.Stat(oldQuintDir); err == nil {
		if _, err := os.Stat(haftDir); os.IsNotExist(err) {
			if renameErr := os.Rename(oldQuintDir, haftDir); renameErr == nil {
				fmt.Println("  ✓ Migrated .quint/ → .haft/")
			}
		}
	}

	_, haftExists := os.Stat(haftDir)

	fmt.Println("Initializing Haft project...")

	if err := createDirectoryStructure(haftDir); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}
	if os.IsNotExist(haftExists) {
		fmt.Println("  ✓ Created .haft/ directory structure")
	} else {
		fmt.Println("  ✓ .haft/ directory structure OK")
	}

	// Create or load project identity
	projCfg, err := project.Create(haftDir, cwd)
	if err != nil {
		return fmt.Errorf("failed to create project identity: %w", err)
	}
	fmt.Printf("  ✓ Project ID: %s (%s)\n", projCfg.ID, projCfg.Name)

	// Determine DB path — unified storage in ~/.haft/projects/{id}/
	unifiedDBPath, err := projCfg.DBPath()
	if err != nil {
		return fmt.Errorf("failed to determine DB path: %w", err)
	}

	// Find the best existing DB: check local paths (old naming conventions)
	var localDB string
	for _, candidate := range []string{
		filepath.Join(haftDir, "haft.db"),
		filepath.Join(haftDir, "quint.db"),
		filepath.Join(cwd, ".quint", "quint.db"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.Size() > 4096 {
			localDB = candidate
			break
		}
	}

	unifiedInfo, _ := os.Stat(unifiedDBPath)
	unifiedEmpty := unifiedInfo == nil || isDBEmpty(unifiedDBPath)

	if localDB != "" && localDB != unifiedDBPath && unifiedEmpty {
		// Migrate local DB to unified storage
		if err := copyFile(localDB, unifiedDBPath); err != nil {
			return fmt.Errorf("failed to migrate database: %w", err)
		}
		fmt.Printf("  ✓ Migrated database from %s\n", filepath.Base(localDB))
		addToGitignore(haftDir, "haft.db")
		addToGitignore(haftDir, "quint.db")
	} else if unifiedInfo == nil {
		// Fresh init — create DB at unified location
		if err := initializeDatabase(unifiedDBPath); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		fmt.Println("  ✓ Initialized database")
	} else {
		// DB already exists at unified location — run migrations
		if err := initializeDatabase(unifiedDBPath); err != nil {
			return fmt.Errorf("failed to update database: %w", err)
		}
		fmt.Println("  ✓ Database OK")
	}

	binaryPath, err := getBinaryPath()
	if err != nil {
		fmt.Printf("  ⚠ Could not determine binary path: %v\n", err)
		binaryPath = "haft"
	}

	if initAll {
		initClaude, initCursor, initGemini, initCodex, initAir = true, true, true, true, true
	}

	if !initClaude && !initCursor && !initGemini && !initCodex && !initAir {
		initClaude = true
	}

	if initClaude {
		if err := configureMCPClaude(cwd, binaryPath); err != nil {
			fmt.Printf("  ⚠ Failed to configure Claude Code MCP: %v\n", err)
		} else {
			fmt.Println("  ✓ Configured MCP for Claude Code (.mcp.json)")
		}
		if destPath, count, err := installCommands(cwd, "claude", initLocal); err != nil {
			fmt.Printf("  ⚠ Failed to install Claude commands: %v\n", err)
		} else {
			fmt.Printf("  ✓ Installed %d slash commands (%s)\n", count, destPath)
		}
		if skillPath, err := installSkill("claude", initLocal, cwd); err != nil {
			fmt.Printf("  ⚠ Failed to install FPF skill: %v\n", err)
		} else if skillPath != "" {
			fmt.Printf("  ✓ Installed /h-reason skill (%s)\n", skillPath)
		}
	}

	if initCursor {
		if err := configureMCPCursor(cwd, binaryPath); err != nil {
			fmt.Printf("  ⚠ Failed to configure Cursor MCP: %v\n", err)
		} else {
			fmt.Println("  ✓ Configured MCP for Cursor (.cursor/mcp.json)")
			fmt.Println("    Note: Make sure haft MCP is enabled in Cursor settings")
		}
		if destPath, count, err := installCommands(cwd, "cursor", initLocal); err != nil {
			fmt.Printf("  ⚠ Failed to install Cursor commands: %v\n", err)
		} else {
			fmt.Printf("  ✓ Installed %d slash commands (%s)\n", count, destPath)
		}
		if skillPath, err := installSkill("cursor", initLocal, cwd); err != nil {
			fmt.Printf("  ⚠ Failed to install FPF skill: %v\n", err)
		} else if skillPath != "" {
			fmt.Printf("  ✓ Installed /h-reason skill (%s)\n", skillPath)
		}
	}

	if initGemini {
		if err := configureMCPGemini(cwd, binaryPath); err != nil {
			fmt.Printf("  ⚠ Failed to configure Gemini CLI MCP: %v\n", err)
		} else {
			fmt.Printf("  ✓ Configured MCP for Gemini CLI (project: %s)\n", cwd)
		}
		if destPath, count, err := installCommands(cwd, "gemini", initLocal); err != nil {
			fmt.Printf("  ⚠ Failed to install Gemini commands: %v\n", err)
		} else {
			fmt.Printf("  ✓ Installed %d slash commands (%s)\n", count, destPath)
		}
	}

	if initCodex || initAir {
		targetName := "Codex CLI"
		switch {
		case initCodex && initAir:
			targetName = "Codex CLI / Air"
		case initAir:
			targetName = "Air"
		}

		if err := configureMCPCodex(cwd, binaryPath); err != nil {
			fmt.Printf("  ⚠ Failed to configure %s MCP: %v\n", targetName, err)
		} else {
			fmt.Printf("  ✓ Configured MCP for %s (project: %s)\n", targetName, cwd)
		}

		// Air currently uses the same Codex prompt/MCP bootstrap.
		if destPath, count, err := installCommands(cwd, "codex", false); err != nil {
			fmt.Printf("  ⚠ Failed to install %s prompts: %v\n", targetName, err)
		} else {
			fmt.Printf("  ✓ Installed %d prompts (%s)\n", count, destPath)
			fmt.Println("    Note: Use /prompts:h-note to invoke")
		}

		if initCodex {
			if skillPath, err := installSkill("codex", false, cwd); err != nil {
				fmt.Printf("  ⚠ Failed to install Codex skill: %v\n", err)
			} else if skillPath != "" {
				fmt.Printf("  ✓ Installed Codex skill $h-reason (%s)\n", skillPath)
			}
		}
		if initAir {
			if skillPath, err := installSkill("air", true, cwd); err != nil {
				fmt.Printf("  ⚠ Failed to install Air skill: %v\n", err)
			} else if skillPath != "" {
				fmt.Printf("  ✓ Installed Air skill h-reason (%s)\n", skillPath)
			}
		}
	}

	fmt.Println("\nInitialization complete!")

	// Check if project already has artifacts
	hasArtifacts := false
	if database, err := db.NewStore(unifiedDBPath); err == nil {
		var count int
		if err := database.GetRawDB().QueryRow("SELECT COUNT(*) FROM artifacts").Scan(&count); err == nil && count > 0 {
			hasArtifacts = true
		}
		_ = database.Close()
	}

	if hasArtifacts {
		fmt.Println("Use /h-status to see active decisions and problems.")
	} else if detectBrownfield(cwd) {
		fmt.Println("\nThis looks like an existing project. Run /h-onboard to discover")
		fmt.Println("existing decisions, architecture docs, ADRs, and build a knowledge map.")
	} else {
		fmt.Println("Use /h-note to capture decisions, /h-reason for structured reasoning.")
	}
	return nil
}

// isDBEmpty checks if a SQLite DB has zero artifacts.
func isDBEmpty(dbPath string) bool {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return true
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM artifacts").Scan(&count); err != nil {
		return true // table doesn't exist = empty
	}
	return count == 0
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func addToGitignore(haftDir, entry string) {
	gitignorePath := filepath.Join(haftDir, ".gitignore")
	content, _ := os.ReadFile(gitignorePath)

	// Check if already present
	if strings.Contains(string(content), entry) {
		return
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	if len(content) > 0 && content[len(content)-1] != '\n' {
		_, _ = f.WriteString("\n")
	}
	_, _ = f.WriteString(entry + "\n")
}

func detectBrownfield(projectRoot string) bool {
	// Check for git history with meaningful commits
	gitDir := filepath.Join(projectRoot, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return false
	}

	// Check for code indicators
	codeIndicators := []string{
		"go.mod", "package.json", "pyproject.toml", "Cargo.toml",
		"pom.xml", "build.gradle", "Makefile", "CMakeLists.txt",
	}
	for _, f := range codeIndicators {
		if _, err := os.Stat(filepath.Join(projectRoot, f)); err == nil {
			return true
		}
	}

	// Check for docs that suggest existing knowledge
	docIndicators := []string{
		"README.md", "docs", "adr", "ARCHITECTURE.md",
	}
	for _, f := range docIndicators {
		if _, err := os.Stat(filepath.Join(projectRoot, f)); err == nil {
			return true
		}
	}

	return false
}

func createDirectoryStructure(haftDir string) error {
	// v5 artifact directories — created minimal, expanded on demand
	dirs := []string{
		"notes",
		"problems",
		"solutions",
		"decisions",
		"evidence",
		"refresh",
	}

	for _, d := range dirs {
		path := filepath.Join(haftDir, d)
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
		gitkeep := filepath.Join(path, ".gitkeep")
		if err := os.WriteFile(gitkeep, []byte(""), 0644); err != nil {
			return err
		}
	}
	return nil
}

func initializeDatabase(dbPath string) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return err
	}
	database, err := db.NewStore(dbPath)
	if err != nil {
		return err
	}
	_ = database.Close()
	return nil
}

func getBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}

type MCPConfig struct {
	MCPServers map[string]MCPServer `json:"mcpServers"`
}

type MCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Cwd     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Timeout int               `json:"timeout,omitempty"`
}

func mergeMCPConfig(configPath, binaryPath, _ string, extraFields map[string]interface{}) error {
	var config MCPConfig

	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("existing config at %s is not valid JSON: %w", configPath, err)
		}
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServer)
	}

	server := MCPServer{
		Command: binaryPath,
		Args:    []string{"serve"},
	}

	if timeout, ok := extraFields["timeout"].(int); ok {
		server.Timeout = timeout
	}
	if env, ok := extraFields["env"].(map[string]string); ok {
		server.Env = env
	}
	if cwd, ok := extraFields["cwd"].(string); ok {
		server.Cwd = cwd
	}

	// Remove old quint-code key if it exists (migration)
	delete(config.MCPServers, "quint-code")

	config.MCPServers["haft"] = server

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func configureMCPClaude(projectRoot, binaryPath string) error {
	configPath := filepath.Join(projectRoot, ".mcp.json")
	return mergeMCPConfig(configPath, binaryPath, projectRoot, map[string]interface{}{
		"env": map[string]string{
			"HAFT_PROJECT_ROOT": projectRoot,
		},
	})
}

func configureMCPCursor(projectRoot, binaryPath string) error {
	configPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	return mergeMCPConfig(configPath, binaryPath, projectRoot, map[string]interface{}{
		"env": map[string]string{
			"HAFT_PROJECT_ROOT": projectRoot,
		},
	})
}

func configureMCPGemini(projectRoot, binaryPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(homeDir, ".gemini", "settings.json")
	return mergeMCPConfig(configPath, binaryPath, projectRoot, map[string]interface{}{
		"timeout": 30000,
		"cwd":     projectRoot,
		"env": map[string]string{
			"HAFT_PROJECT_ROOT": projectRoot,
		},
	})
}

func configureMCPCodex(projectRoot, binaryPath string) error {
	configPath := filepath.Join(projectRoot, ".codex", "config.toml")

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	existing := ""
	if data, err := os.ReadFile(configPath); err == nil {
		existing = string(data)
	}

	tomlSection := fmt.Sprintf(`
[mcp_servers.haft]
command = "%s"
args = ["serve"]
startup_timeout_sec = 10
tool_timeout_sec = 60

[mcp_servers.haft.env]
HAFT_PROJECT_ROOT = "%s"
`, binaryPath, projectRoot)

	// Remove old quint-code section if present
	if start := strings.Index(existing, "[mcp_servers.quint-code]"); start != -1 {
		end := len(existing)
		if nextSection := strings.Index(existing[start+1:], "\n["); nextSection != -1 {
			end = start + 1 + nextSection
		}
		existing = existing[:start] + existing[end:]
	}

	if start := strings.Index(existing, "[mcp_servers.haft]"); start != -1 {
		end := len(existing)
		if nextSection := strings.Index(existing[start+1:], "\n["); nextSection != -1 {
			end = start + 1 + nextSection
		}
		existing = existing[:start] + existing[end:]
	}

	updated := strings.TrimRight(existing, "\n") + tomlSection

	return os.WriteFile(configPath, []byte(updated), 0644)
}
