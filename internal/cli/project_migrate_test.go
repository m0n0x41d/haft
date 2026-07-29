package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestProjectMigrateCommandRequiresExactRootAndIdentity(t *testing.T) {
	previousRoot := projectMigrateRoot
	previousID := projectMigrateID
	projectMigrateRoot = ""
	projectMigrateID = ""
	t.Cleanup(func() {
		projectMigrateRoot = previousRoot
		projectMigrateID = previousID
	})

	command := &cobra.Command{}
	command.SetOut(&bytes.Buffer{})
	err := runProjectMigrate(command, nil)
	if err == nil || !strings.Contains(err.Error(), "--project-root is required") {
		t.Fatalf("missing-root error = %v", err)
	}

	projectMigrateRoot = t.TempDir()
	err = runProjectMigrate(command, nil)
	if err == nil || !strings.Contains(err.Error(), "--project-id is required") {
		t.Fatalf("missing-id error = %v", err)
	}
}

func TestProjectMigrateCommandExposesNoHostSelectionFlags(t *testing.T) {
	if projectMigrateCmd.Flags().Lookup("project-root") == nil {
		t.Fatal("project migrate is missing --project-root")
	}
	if projectMigrateCmd.Flags().Lookup("project-id") == nil {
		t.Fatal("project migrate is missing --project-id")
	}
	for _, forbidden := range []string{
		"agents",
		"claude",
		"codex",
		"host",
		"local",
	} {
		if projectMigrateCmd.Flags().Lookup(forbidden) != nil {
			t.Fatalf("project migrate exposes host flag --%s", forbidden)
		}
	}
}
