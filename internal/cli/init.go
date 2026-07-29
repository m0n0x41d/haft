package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/overseer"

	"github.com/spf13/cobra"
)

var (
	initClaude                  bool
	initCursor                  bool
	initGemini                  bool
	initCodex                   bool
	initAgents                  bool
	initMCPOnly                 bool
	initAir                     bool
	initOpencode                bool
	initHermes                  bool
	initZed                     bool
	initAgy                     bool
	initGrok                    bool
	initAll                     bool
	initCoreOnly                bool
	initLocal                   bool
	initNoFileInstructions      bool
	initScopeID                 string
	initOverseer                bool
	initOverseerReviewer        string
	initOverseerReviewerCommand string
	initOverseerReviewOnHook    bool
	initOverseerReviewTimeout   int
)

type initHostOptions struct {
	claude   bool
	cursor   bool
	gemini   bool
	codex    bool
	air      bool
	opencode bool
	hermes   bool
	zed      bool
	agy      bool
	pi       bool
	grok     bool
	all      bool
}

type initSyntaxError struct {
	cause error
}

func (failure initSyntaxError) Error() string {
	return failure.cause.Error() +
		"\nRun 'haft init --help' for help."
}

func (failure initSyntaxError) Unwrap() error {
	return failure.cause
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize FPF project structure and MCP configuration",
	Args:  validateInitPositionalArgs,
	Long: `Initialize a new Haft project in the current directory.

With no flags in an interactive terminal, this command opens a host
multi-select. No host is preselected. With any explicit flag, the menu is
skipped and the requested non-interactive initialization runs directly.
Bare non-interactive invocation fails before writing anything.

This command creates:
  - Core .haft/ structure, project identity, config, and database
  - Spec carriers and SWE MethodPack only when the admitted scope marks them Required
  - MCP configuration for selected host agents
  - Host-specific skills when the selected host owns them
  - Independent .agents/skills publication when --agents is explicit
  - Cleanup of legacy Haft slash commands and prompt files
  - Experimental legacy host configs when explicitly requested

Examples:
  haft init              # Interactive host multi-select (TTY only)
  haft init --core-only  # Initialize/migrate only the project core
  haft init --claude     # Claude MCP + global skills + CLAUDE.md instructions
  haft init --claude --local # Claude MCP + project skills + CLAUDE.md
  haft init --codex      # Codex MCP + global skills + AGENTS.md instructions
  haft init --codex --mcp-only # Codex MCP configuration only
  haft init --agents     # Global .agents/skills only
  haft init --codex --agents # Codex full integration + .agents/skills
  haft init --scope-id software # Select one exact admitted scope in a mixed project
  haft init --overseer   # Enable soft local overseer post-commit packet loop
  haft init --codex --overseer
  haft overseer init     # Enable overseer later, auto-detecting the project agent
  haft init --opencode   # OpenCode MCP + skills (sst/opencode)
  haft init --hermes     # Hermes MCP + external skills directory
  haft init --zed        # Zed global MCP context server
  haft init --agy        # Google Antigravity shared MCP config
  haft init --pi         # Pi — bundled @haft/pi package + .pi/settings.json entry
  haft init --grok       # Grok CLI project .grok/config.toml MCP + skills
  haft init --all        # Full Claude + Codex integrations
  haft init --cursor     # Experimental Cursor config
  haft init --gemini     # Experimental Gemini CLI config
  haft init --air        # Experimental Air skill + Codex-compatible prompts/MCP`,
	RunE: runPublicInit,
}

func init() {
	initCmd.SetFlagErrorFunc(renderInitFlagSyntaxError)
	initCmd.Flags().BoolVar(&initClaude, "claude", false, "Configure stable Claude Code integration")
	initCmd.Flags().BoolVar(&initCursor, "cursor", false, "Configure experimental Cursor MCP")
	initCmd.Flags().BoolVar(&initGemini, "gemini", false, "Configure experimental Gemini CLI MCP")
	initCmd.Flags().BoolVar(&initCodex, "codex", false, "Configure stable Codex CLI integration")
	initCmd.Flags().BoolVar(&initAgents, "agents", false, "Install the Haft skill bundle under .agents/skills")
	initCmd.Flags().BoolVar(&initMCPOnly, "mcp-only", false, "Configure only MCP for explicitly selected hosts (compatibility mode)")
	initCmd.Flags().BoolVar(&initOpencode, "opencode", false, "Configure for OpenCode (sst/opencode) — writes opencode.json with the Haft MCP server and installs skills")
	initCmd.Flags().BoolVar(&initAir, "air", false, "Configure experimental JetBrains Air integration")
	initCmd.Flags().BoolVar(&initAll, "all", false, "Configure all stable hosts (Claude + Codex)")
	initCmd.Flags().BoolVar(&initCoreOnly, "core-only", false, "Initialize or migrate the Haft project core without host files")
	initCmd.Flags().BoolVar(&initLocal, "local", false, "Install skills in the project directory instead of globally")
	initCmd.Flags().BoolVar(&initNoFileInstructions, "no-file-instructions", false, "Skip installing/updating project instruction files")
	initCmd.Flags().BoolVar(&initNoFileInstructions, "no-claude-md", false, "Deprecated alias for --no-file-instructions")
	initCmd.Flags().StringVar(&initScopeID, "scope-id", "", "Exact admitted ScopeID when the canonical project profile has multiple scopes")
	initCmd.Flags().BoolVar(&initOverseer, "overseer", false, "Install opt-in soft overseer post-commit hook and config")
	initCmd.Flags().StringVar(&initOverseerReviewer, "overseer-reviewer", overseer.ReviewerAuto, "Overseer reviewer preset: auto, manual, command, codex, or claude")
	initCmd.Flags().StringVar(&initOverseerReviewerCommand, "overseer-reviewer-command", "", "Override command used by overseer reviewer; receives HAFT_OVERSEER_* env vars")
	initCmd.Flags().BoolVar(&initOverseerReviewOnHook, "overseer-review-on-hook", false, "Run configured overseer reviewer from the post-commit hook")
	initCmd.Flags().IntVar(&initOverseerReviewTimeout, "overseer-review-timeout", 180, "Overseer reviewer timeout in seconds")
	_ = initCmd.Flags().MarkDeprecated("no-claude-md", "use --no-file-instructions")

	rootCmd.AddCommand(initCmd)
}

func validateInitPositionalArgs(
	_ *cobra.Command,
	args []string,
) error {
	if len(args) == 0 {
		return nil
	}
	received := strings.Join(args, " ")
	cause := fmt.Errorf(
		"haft init accepts no positional arguments: %q",
		received,
	)
	return renderInitSyntaxError(cause)
}

func renderInitFlagSyntaxError(
	_ *cobra.Command,
	cause error,
) error {
	return renderInitSyntaxError(cause)
}

func renderInitSyntaxError(cause error) error {
	return initSyntaxError{cause: cause}
}

func hasInitHost(options initHostOptions) bool {
	return options.claude ||
		options.cursor ||
		options.gemini ||
		options.codex ||
		options.air ||
		options.opencode ||
		options.hermes ||
		options.zed ||
		options.agy ||
		options.pi ||
		options.grok ||
		options.all
}

func initCommandContext(cmd *cobra.Command) context.Context {
	if cmd == nil || cmd.Context() == nil {
		return context.Background()
	}
	return cmd.Context()
}

func initOutputWriter(cmd *cobra.Command) io.Writer {
	if cmd == nil {
		return os.Stdout
	}
	return cmd.OutOrStdout()
}

type overseerSetupOptions struct {
	reviewer     string
	command      string
	reviewOnHook bool
	timeout      int
	hosts        initHostOptions
}

func buildOverseerConfigForProject(projectRoot string, options overseerSetupOptions) (overseer.Config, error) {
	reviewer := strings.TrimSpace(options.reviewer)
	command := strings.TrimSpace(options.command)
	if reviewer == "" {
		reviewer = overseer.ReviewerAuto
	}
	if reviewer == overseer.ReviewerAuto {
		reviewer = inferOverseerReviewerAgent(projectRoot, options.hosts)
	}
	if reviewer == overseer.ReviewerManual && command != "" {
		reviewer = overseer.ReviewerCommand
	}
	reviewOnHook := options.reviewOnHook || reviewer != overseer.ReviewerManual
	return overseer.ConfigForReviewer(reviewer, command, reviewOnHook, options.timeout)
}

func inferOverseerReviewerAgent(projectRoot string, hosts initHostOptions) string {
	if hosts.codex || hosts.air {
		return overseer.ReviewerCodex
	}
	if hosts.claude {
		return overseer.ReviewerClaude
	}
	if pathExists(filepath.Join(projectRoot, ".codex", "config.toml")) {
		return overseer.ReviewerCodex
	}
	if pathExists(filepath.Join(projectRoot, ".mcp.json")) || pathExists(filepath.Join(projectRoot, "CLAUDE.md")) {
		return overseer.ReviewerClaude
	}
	return overseer.ReviewerManual
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func configureOverseerWithConfig(
	projectRoot string,
	binaryPath string,
	configFactory func() (overseer.Config, error),
) (overseer.Config, overseer.HookInstallResult, error) {
	config, err := configFactory()
	if err != nil {
		return overseer.Config{}, overseer.HookInstallResult{}, err
	}
	if err := overseer.SaveConfig(projectRoot, config); err != nil {
		return overseer.Config{}, overseer.HookInstallResult{}, err
	}
	result, err := overseer.InstallPostCommitHook(projectRoot, binaryPath)
	return config, result, err
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

const (
	claudeProjectRootEnv = "${PWD:-.}"
	cursorProjectRootEnv = "${workspaceFolder}"
	codexProjectRootEnv  = "."
)
