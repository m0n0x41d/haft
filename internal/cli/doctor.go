package cli

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/config"
	"github.com/m0n0x41d/haft/internal/hooks"
	"github.com/m0n0x41d/haft/internal/skills"
)

var (
	doctorMovedFrom string
	doctorRepair    bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check haft installation health",
	Long:  "Verify configuration, auth, runtime dependencies, and project setup.",
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().StringVar(&doctorMovedFrom, "moved-from", "", "old project root to audit after a repository move")
	doctorCmd.Flags().BoolVar(&doctorRepair, "repair", false, "repair exact stale project-root carriers found by --moved-from")

	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(_ *cobra.Command, _ []string) error {
	if doctorRepair && strings.TrimSpace(doctorMovedFrom) == "" {
		return fmt.Errorf("--repair requires --moved-from")
	}

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

	check("Host JS runtime", func() (string, error) {
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

	warn("Brave Search key", func() (string, bool) {
		if key := os.Getenv("BRAVE_SEARCH_API_KEY"); key != "" {
			return "set via BRAVE_SEARCH_API_KEY", true
		}
		return "not set (web_search disabled)", false
	})

	// --- Project ---
	fmt.Println()
	fmt.Println("Project:")
	binding, err := resolveProjectBinding()
	if err != nil {
		fmt.Printf("  \u2717 Project binding: %s\n", projectBindingError(binding, err))
		failed++
	} else {
		fmt.Printf("  \u2713 Project root: %s\n", binding.ProjectRoot)
		passed++

		check("Project config", func() (string, error) {
			return fmt.Sprintf("ok id=%s name=%s", binding.ProjectID, binding.ProjectName), nil
		})

		check("Project binding", func() (string, error) {
			return formatProjectBindingDiagnostic(binding), nil
		})

		check("Database", func() (string, error) {
			db, err := sql.Open("sqlite", binding.DBPath)
			if err != nil {
				return "", err
			}
			defer db.Close()
			if err := db.Ping(); err != nil {
				return "", err
			}
			return fmt.Sprintf("%s (%s, artifacts=%d)", binding.DBPath, binding.DBState, binding.ArtifactCount), nil
		})

		warn("Hooks", func() (string, bool) {
			exec := hooks.NewExecutor(binding.ProjectRoot)
			if exec.HasHooks() {
				return "configured", true
			}
			return "none configured", true // not a failure
		})

		warn("Skills", func() (string, bool) {
			loader := skills.NewLoader(binding.ProjectRoot)
			list := loader.List()
			if len(list) > 0 {
				return fmt.Sprintf("%d loaded", len(list)), true
			}
			return "none loaded", true // not a failure
		})

		warn("haft serve processes", func() (string, bool) {
			return doctorServeProcessStatus(
				binding.ProjectRoot,
				collectDoctorServeProcessSnapshot(),
			)
		})

		if strings.TrimSpace(doctorMovedFrom) != "" {
			fmt.Println()
			fmt.Println("Project Path Carriers:")
			results, err := runDoctorProjectPathCarrierCheck(binding)
			if err != nil {
				fmt.Printf("  ✗ Moved root audit: %s\n", err)
				failed++
			} else {
				for _, result := range results {
					if result.Changed {
						fmt.Printf("  ✓ %s: repaired %d stale reference(s) (%s)\n", result.Label, result.Occurrences, result.Path)
						passed++
						continue
					}
					if result.Occurrences == 0 {
						fmt.Printf("  ✓ %s: clean (%s)\n", result.Label, result.Path)
						passed++
						continue
					}
					if result.Repairable && !doctorRepair {
						fmt.Printf("  ⚠ %s: %d stale reference(s), repairable with --repair (%s)\n", result.Label, result.Occurrences, result.Path)
						warned++
						continue
					}
					if result.Repairable {
						fmt.Printf("  ⚠ %s: %d stale reference(s), not changed: %s (%s)\n", result.Label, result.Occurrences, result.Message, result.Path)
						warned++
						continue
					}
					fmt.Printf("  ⚠ %s: %d stale reference(s), manual review required (%s)\n", result.Label, result.Occurrences, result.Path)
					warned++
				}
			}
		}
	}

	// --- Summary ---
	fmt.Println()
	fmt.Printf("Result: %d passed, %d warnings, %d failed\n", passed, warned, failed)
	if failed > 0 {
		return fmt.Errorf("%d checks failed", failed)
	}
	return nil
}

func runDoctorProjectPathCarrierCheck(binding ProjectBinding) ([]projectPathCarrierResult, error) {
	oldRoot, err := filepath.Abs(strings.TrimSpace(doctorMovedFrom))
	if err != nil {
		return nil, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	carriers := projectPathCarrierCandidates(homeDir, binding.ProjectRoot)
	audit, err := auditProjectPathCarriers(carriers, oldRoot)
	if err != nil {
		return nil, err
	}
	if !doctorRepair {
		return audit, nil
	}

	return repairProjectPathCarriers(audit, oldRoot, binding.ProjectRoot)
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
