package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"quint-mcp/db"
)

var (
	initClaude bool
	initCursor bool
	initGemini bool
	initCodex  bool
	initAll    bool
	initLocal  bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize FPF project structure and MCP configuration",
	Long: `Initialize a new Quint Code project in the current directory.

This command creates:
  - .quint/ directory structure (knowledge base, evidence, decisions)
  - MCP configuration for selected AI tools
  - Slash commands (global by default, or local with --local)

Examples:
  quint-code init              # Claude, global commands (~/.claude/commands/)
  quint-code init --local      # Claude, local commands (.claude/commands/)
  quint-code init --all        # All tools, global commands
  quint-code init --cursor     # Cursor only`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initClaude, "claude", false, "Configure for Claude Code")
	initCmd.Flags().BoolVar(&initCursor, "cursor", false, "Configure for Cursor")
	initCmd.Flags().BoolVar(&initGemini, "gemini", false, "Configure for Gemini CLI")
	initCmd.Flags().BoolVar(&initCodex, "codex", false, "Configure for Codex CLI")
	initCmd.Flags().BoolVar(&initAll, "all", false, "Configure for all supported tools")
	initCmd.Flags().BoolVar(&initLocal, "local", false, "Install commands in project directory instead of global")

	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	quintDir := filepath.Join(cwd, ".quint")
	dbPath := filepath.Join(quintDir, "quint.db")

	_, quintExists := os.Stat(quintDir)
	_, dbExists := os.Stat(dbPath)

	fmt.Println("Initializing Quint Code project...")

	if err := createDirectoryStructure(quintDir); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}
	if os.IsNotExist(quintExists) {
		fmt.Println("  ✓ Created .quint/ directory structure")
	} else {
		fmt.Println("  ✓ .quint/ directory structure OK")
	}

	if err := initializeDatabase(quintDir); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	if os.IsNotExist(dbExists) {
		fmt.Println("  ✓ Initialized database")
	} else {
		fmt.Println("  ✓ Database OK")
	}

	binaryPath, err := getBinaryPath()
	if err != nil {
		fmt.Printf("  ⚠ Could not determine binary path: %v\n", err)
		binaryPath = "quint-code"
	}

	if initAll {
		initClaude, initCursor, initGemini, initCodex = true, true, true, true
	}

	if !initClaude && !initCursor && !initGemini && !initCodex {
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
	}

	if initCursor {
		if err := configureMCPCursor(cwd, binaryPath); err != nil {
			fmt.Printf("  ⚠ Failed to configure Cursor MCP: %v\n", err)
		} else {
			fmt.Println("  ✓ Configured MCP for Cursor (.cursor/mcp.json)")
			fmt.Println("    Note: Make sure quint-code MCP is enabled in Cursor settings")
		}
		if destPath, count, err := installCommands(cwd, "cursor", initLocal); err != nil {
			fmt.Printf("  ⚠ Failed to install Cursor commands: %v\n", err)
		} else {
			fmt.Printf("  ✓ Installed %d slash commands (%s)\n", count, destPath)
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

	if initCodex {
		if err := configureMCPCodex(cwd, binaryPath); err != nil {
			fmt.Printf("  ⚠ Failed to configure Codex CLI MCP: %v\n", err)
		} else {
			fmt.Printf("  ✓ Configured MCP for Codex CLI (project: %s)\n", cwd)
		}
		// Codex only supports global prompts
		if destPath, count, err := installCommands(cwd, "codex", false); err != nil {
			fmt.Printf("  ⚠ Failed to install Codex prompts: %v\n", err)
		} else {
			fmt.Printf("  ✓ Installed %d prompts (%s)\n", count, destPath)
			fmt.Println("    Note: Use /prompts:q0-init to invoke")
		}
	}

	fmt.Println("\nInitialization complete! Run /q0-init to start.")
	return nil
}

func createDirectoryStructure(quintDir string) error {
	dirs := []string{
		"evidence",
		"decisions",
		"sessions",
		"knowledge/L0",
		"knowledge/L1",
		"knowledge/L2",
		"knowledge/invalid",
		"agents",
	}

	for _, d := range dirs {
		path := filepath.Join(quintDir, d)
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

func initializeDatabase(quintDir string) error {
	dbPath := filepath.Join(quintDir, "quint.db")
	database, err := db.New(dbPath)
	if err != nil {
		return err
	}
	database.Close()
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

func mergeMCPConfig(configPath, binaryPath, projectRoot string, extraFields map[string]interface{}) error {
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
		Cwd:     projectRoot,
		Env: map[string]string{
			"QUINT_PROJECT_ROOT": projectRoot,
		},
	}

	if timeout, ok := extraFields["timeout"].(int); ok {
		server.Timeout = timeout
	}

	config.MCPServers["quint-code"] = server

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
	return mergeMCPConfig(configPath, binaryPath, projectRoot, nil)
}

func configureMCPCursor(projectRoot, binaryPath string) error {
	configPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	return mergeMCPConfig(configPath, binaryPath, projectRoot, nil)
}

func configureMCPGemini(projectRoot, binaryPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(homeDir, ".gemini", "settings.json")
	return mergeMCPConfig(configPath, binaryPath, projectRoot, map[string]interface{}{
		"timeout": 30000,
	})
}

func configureMCPCodex(projectRoot, binaryPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(homeDir, ".codex", "config.toml")

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	existing := ""
	if data, err := os.ReadFile(configPath); err == nil {
		existing = string(data)
	}

	tomlSection := fmt.Sprintf(`
[mcp_servers.quint-code]
command = "%s"
args = ["serve"]
env = { QUINT_PROJECT_ROOT = "%s" }
`, binaryPath, projectRoot)

	if start := strings.Index(existing, "[mcp_servers.quint-code]"); start != -1 {
		end := len(existing)
		if nextSection := strings.Index(existing[start+1:], "\n["); nextSection != -1 {
			end = start + 1 + nextSection
		}
		existing = existing[:start] + existing[end:]
	}

	updated := strings.TrimRight(existing, "\n") + tomlSection

	return os.WriteFile(configPath, []byte(updated), 0644)
}

