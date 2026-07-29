package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func runTypedCoreInitForTest(t *testing.T) {
	t.Helper()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve test HOME: %v", err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("create test HOME: %v", err)
	}

	restore := captureInitHostFlagState()
	defer restore.apply()
	clearInitHostFlags()
	initCoreOnly = true

	command := &cobra.Command{}
	command.SetIn(strings.NewReader(""))
	command.SetOut(io.Discard)
	var coreOnly bool
	command.Flags().BoolVar(&coreOnly, "core-only", false, "")
	if err := command.Flags().Set("core-only", "true"); err != nil {
		t.Fatalf("set core-only init flag: %v", err)
	}
	if err := runPublicInit(command, nil); err != nil {
		t.Fatalf("run typed core init: %v", err)
	}
}

func physicalInitTestTempDir(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve physical test directory: %v", err)
	}
	return path
}
