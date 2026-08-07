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
	"github.com/m0n0x41d/haft/internal/embedding"
)

var (
	doctorMovedFrom string
	doctorRepair    bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check haft installation health",
	Long:  "Verify the current CLI, project binding, database, and MCP process setup.",
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

	check("Git", func() (string, error) {
		path, err := exec.LookPath("git")
		if err != nil {
			return "", fmt.Errorf("git not found in PATH")
		}
		out, _ := exec.Command(path, "--version").Output()
		return trimOutput(out), nil
	})

	fmt.Println()
	fmt.Println("Configuration:")
	check("Embedding config", doctorEmbeddingConfigStatus)

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

func doctorEmbeddingConfigStatus() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("load global config: %w", err)
	}

	provider := strings.TrimSpace(cfg.Embedding.Provider)
	if err := embedding.ValidateProvider(provider); err != nil {
		return "", err
	}
	if provider == "" {
		provider = "local (default)"
	}

	model := strings.TrimSpace(cfg.Embedding.Model)
	if model == "" {
		model = "default"
	}

	return fmt.Sprintf(
		"provider=%s model=%s dim=%d",
		provider,
		model,
		cfg.Embedding.Dim,
	), nil
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
