package cli

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/config"
	"github.com/m0n0x41d/haft/internal/hooks"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/skills"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check haft installation health",
	Long:  "Verify configuration, auth, runtime dependencies, and project setup.",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(_ *cobra.Command, _ []string) error {
	fmt.Println("haft doctor")
	fmt.Println()

	passed := 0
	failed := 0
	warned := 0

	check := func(name string, fn func() (string, error)) {
		result, err := fn()
		if err != nil {
			fmt.Printf("  \u2717 %s: %s\n", name, err)
			failed++
		} else {
			fmt.Printf("  \u2713 %s: %s\n", name, result)
			passed++
		}
	}

	warn := func(name string, fn func() (string, bool)) {
		result, ok := fn()
		if !ok {
			fmt.Printf("  \u26A0 %s: %s\n", name, result)
			warned++
		} else {
			fmt.Printf("  \u2713 %s: %s\n", name, result)
			passed++
		}
	}

	// --- System ---
	fmt.Println("System:")
	check("Platform", func() (string, error) {
		return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), nil
	})

	check("TUI runtime", func() (string, error) {
		if path, err := exec.LookPath("bun"); err == nil {
			out, _ := exec.Command(path, "--version").Output()
			return fmt.Sprintf("bun %s (%s)", trimOutput(out), path), nil
		}
		if path, err := exec.LookPath("node"); err == nil {
			out, _ := exec.Command(path, "--version").Output()
			return fmt.Sprintf("node %s (%s)", trimOutput(out), path), nil
		}
		return "", fmt.Errorf("bun or node not found in PATH")
	})

	check("Git", func() (string, error) {
		path, err := exec.LookPath("git")
		if err != nil {
			return "", fmt.Errorf("git not found in PATH")
		}
		out, _ := exec.Command(path, "--version").Output()
		return trimOutput(out), nil
	})

	fmt.Println()

	// --- Config ---
	fmt.Println("Configuration:")
	check("Config file", func() (string, error) {
		cfg, err := config.Load()
		if err != nil {
			return "", fmt.Errorf("cannot load: %v", err)
		}
		return fmt.Sprintf("model=%s", cfg.Model), nil
	})

	// --- Auth ---
	fmt.Println()
	fmt.Println("Authentication:")
	warn("OpenAI API key", func() (string, bool) {
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			return "set via OPENAI_API_KEY", true
		}
		return "not set (use OPENAI_API_KEY or 'haft login')", false
	})

	warn("Codex OAuth", func() (string, bool) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "cannot determine home dir", false
		}
		// Check haft auth
		if _, err := os.Stat(filepath.Join(home, ".config", "haft", "auth.json")); err == nil {
			return "~/.config/haft/auth.json found", true
		}
		// Check codex CLI auth
		if _, err := os.Stat(filepath.Join(home, ".codex", "auth.json")); err == nil {
			return "~/.codex/auth.json found (Codex CLI)", true
		}
		return "no OAuth tokens (run 'haft login')", false
	})

	warn("Anthropic API key", func() (string, bool) {
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			return "set via ANTHROPIC_API_KEY", true
		}
		return "not set", false
	})

	warn("Claude Code CLI", func() (string, bool) {
		path, err := exec.LookPath("claude")
		if err != nil {
			return "not found in PATH (install from https://docs.claude.com/en/docs/claude-code to use model=claude-code)", false
		}
		out, _ := exec.Command(path, "--version").Output()
		v := trimOutput(out)
		if v == "" {
			v = "detected"
		}
		return fmt.Sprintf("%s (%s)", v, path), true
	})

	warn("Brave Search key", func() (string, bool) {
		if key := os.Getenv("BRAVE_SEARCH_API_KEY"); key != "" {
			return "set via BRAVE_SEARCH_API_KEY", true
		}
		return "not set (web_search disabled)", false
	})

	// --- Project ---
	fmt.Println()
	fmt.Println("Project:")
	projectRoot, err := findProjectRoot()
	if err != nil {
		fmt.Printf("  \u2717 Project root: not found (run 'haft init')\n")
		failed++
	} else {
		fmt.Printf("  \u2713 Project root: %s\n", projectRoot)
		passed++

		haftDir := filepath.Join(projectRoot, ".haft")

		check("Project config", func() (string, error) {
			cfg, err := project.Load(haftDir)
			if err != nil || cfg == nil {
				return "", fmt.Errorf("not initialized (run 'haft init')")
			}
			return "ok", nil
		})

		check("Database", func() (string, error) {
			projCfg, _ := project.Load(haftDir)
			if projCfg == nil {
				return "", fmt.Errorf("no project config")
			}
			dbPath, err := projCfg.DBPath()
			if err != nil {
				return "", err
			}
			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				return "", err
			}
			defer db.Close()
			if err := db.Ping(); err != nil {
				return "", err
			}
			return dbPath, nil
		})

		check("TUI entry", func() (string, error) {
			entry, err := findTUIEntry(projectRoot)
			if err != nil {
				return "", err
			}
			return entry, nil
		})

		warn("Hooks", func() (string, bool) {
			exec := hooks.NewExecutor(projectRoot)
			if exec.HasHooks() {
				return "configured", true
			}
			return "none configured", true // not a failure
		})

		warn("Skills", func() (string, bool) {
			loader := skills.NewLoader(projectRoot)
			list := loader.List()
			if len(list) > 0 {
				return fmt.Sprintf("%d loaded", len(list)), true
			}
			return "none loaded", true // not a failure
		})
	}

	// --- Summary ---
	fmt.Println()
	fmt.Printf("Result: %d passed, %d warnings, %d failed\n", passed, warned, failed)
	if failed > 0 {
		return fmt.Errorf("%d checks failed", failed)
	}
	return nil
}

func trimOutput(b []byte) string {
	s := string(b)
	if len(s) > 50 {
		s = s[:50]
	}
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
