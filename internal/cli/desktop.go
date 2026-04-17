package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var desktopCmd = &cobra.Command{
	Use:   "desktop",
	Short: "Launch the Haft desktop app",
	Long: `Launch the Haft desktop application.

Search order:
  1. ~/Applications/Haft.app (macOS)
  2. haft-desktop in PATH
  3. ~/.haft/bin/haft-desktop
  4. desktop-tauri/target/release/bundle/ in the project directory (dev)`,
	RunE: runDesktop,
}

func init() {
	rootCmd.AddCommand(desktopCmd)
}

func runDesktop(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()

	// Resolve project root to pass to the desktop app via env var.
	projectRoot := ""
	if pr, err := findProjectRoot(); err == nil {
		projectRoot = pr
	}

	// On macOS, prefer the .app bundle — it integrates with Spotlight, Dock, etc.
	if runtime.GOOS == "darwin" && home != "" {
		appPaths := []string{
			filepath.Join(home, "Applications", "Haft.app"),
			"/Applications/Haft.app",
		}

		if projectRoot != "" {
			appPaths = append(appPaths, filepath.Join(projectRoot, "desktop-tauri", "target", "release", "bundle", "macos", "Haft.app"))
		}

		for _, appPath := range appPaths {
			binaryPath := filepath.Join(appPath, "Contents", "MacOS", "Haft")
			if _, err := os.Stat(binaryPath); err == nil {
				fmt.Printf("%s⟳ Launching: %s%s\n", aCyan, appPath, aReset)
				appCmd := exec.Command(binaryPath)
				appCmd.Stdout = os.Stdout
				appCmd.Stderr = os.Stderr
				appCmd.Stdin = os.Stdin
				if projectRoot != "" {
					appCmd.Env = append(os.Environ(), "HAFT_PROJECT_ROOT="+projectRoot)
				}
				return appCmd.Start()
			}
		}
	}

	// Fall back to direct binary search (Linux, or macOS without .app)
	candidates := []string{}

	if p, err := exec.LookPath("haft-desktop"); err == nil {
		candidates = append(candidates, p)
	}

	if home != "" {
		userBin := filepath.Join(home, ".haft", "bin", "haft-desktop")
		if _, err := os.Stat(userBin); err == nil {
			candidates = append(candidates, userBin)
		}
	}

	if projectRoot != "" {
		localBuild := filepath.Join(projectRoot, "desktop-tauri", "target", "release", "haft-desktop")
		if _, err := os.Stat(localBuild); err == nil {
			candidates = append(candidates, localBuild)
		}
	}

	if len(candidates) == 0 {
		fmt.Println("Desktop app not found.")
		fmt.Println()
		fmt.Println("Install it with:")
		fmt.Println("  task desktop:install  # builds and installs to ~/Applications")
		return nil
	}

	binary := candidates[0]
	fmt.Printf("%s⟳ Launching: %s%s\n", aCyan, binary, aReset)

	desktop := exec.Command(binary)
	desktop.Stdout = os.Stdout
	desktop.Stderr = os.Stderr
	desktop.Stdin = os.Stdin
	if projectRoot != "" {
		desktop.Env = append(os.Environ(), "HAFT_PROJECT_ROOT="+projectRoot)
	}

	return desktop.Run()
}
