package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var desktopCmd = &cobra.Command{
	Use:   "desktop",
	Short: "Launch the Haft desktop app",
	Long: `Launch the Haft desktop application (Wails v2).

Looks for the desktop binary in standard locations:
  1. haft-desktop in PATH
  2. desktop/build/bin/ in the project directory
  3. ~/.haft/bin/haft-desktop

If not found, suggests how to build it.`,
	RunE: runDesktop,
}

func init() {
	rootCmd.AddCommand(desktopCmd)
}

func runDesktop(cmd *cobra.Command, args []string) error {
	// Search for desktop binary
	candidates := []string{}

	// 1. haft-desktop in PATH
	if p, err := exec.LookPath("haft-desktop"); err == nil {
		candidates = append(candidates, p)
	}

	// 2. Project-local build
	if projectRoot, err := findProjectRoot(); err == nil {
		localBuild := filepath.Join(projectRoot, "desktop", "build", "bin", "haft-desktop")
		if _, err := os.Stat(localBuild); err == nil {
			candidates = append(candidates, localBuild)
		}
		// macOS .app bundle
		macApp := filepath.Join(projectRoot, "desktop", "build", "bin", "haft-desktop.app", "Contents", "MacOS", "haft-desktop")
		if _, err := os.Stat(macApp); err == nil {
			candidates = append(candidates, macApp)
		}
	}

	// 3. User-local install
	if home, err := os.UserHomeDir(); err == nil {
		userBin := filepath.Join(home, ".haft", "bin", "haft-desktop")
		if _, err := os.Stat(userBin); err == nil {
			candidates = append(candidates, userBin)
		}
	}

	if len(candidates) == 0 {
		fmt.Println("Desktop app not found.")
		fmt.Println()
		fmt.Println("Build it with:")
		fmt.Println("  task desktop:build    # requires Wails v2")
		fmt.Println()
		fmt.Println("Or run in dev mode:")
		fmt.Println("  task desktop          # wails dev with hot reload")
		return nil
	}

	binary := candidates[0]
	fmt.Printf("%s⟳ Launching desktop: %s%s\n", aCyan, binary, aReset)

	desktop := exec.Command(binary)
	desktop.Stdout = os.Stdout
	desktop.Stderr = os.Stderr
	desktop.Stdin = os.Stdin

	return desktop.Run()
}
