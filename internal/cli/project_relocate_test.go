package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestProjectRelocateCommandRequiresBothRootsAndIdentity(t *testing.T) {
	previousFrom := projectRelocateFromRoot
	previousTo := projectRelocateToRoot
	previousID := projectRelocateID
	projectRelocateFromRoot = ""
	projectRelocateToRoot = ""
	projectRelocateID = ""
	t.Cleanup(func() {
		projectRelocateFromRoot = previousFrom
		projectRelocateToRoot = previousTo
		projectRelocateID = previousID
	})

	command := &cobra.Command{}
	command.SetOut(&bytes.Buffer{})
	err := runProjectRelocate(command, nil)
	if err == nil || !strings.Contains(err.Error(), "--from-root is required") {
		t.Fatalf("missing-from-root error = %v", err)
	}

	projectRelocateFromRoot = "/previous/project"
	err = runProjectRelocate(command, nil)
	if err == nil || !strings.Contains(err.Error(), "--project-root is required") {
		t.Fatalf("missing-current-root error = %v", err)
	}

	projectRelocateToRoot = t.TempDir()
	err = runProjectRelocate(command, nil)
	if err == nil || !strings.Contains(err.Error(), "--project-id is required") {
		t.Fatalf("missing-project-id error = %v", err)
	}
}

func TestProjectRelocateCommandExposesOnlyExactRelocationFlags(t *testing.T) {
	t.Parallel()

	for _, required := range []string{"from-root", "project-root", "project-id"} {
		if projectRelocateCmd.Flags().Lookup(required) == nil {
			t.Fatalf("project relocate is missing --%s", required)
		}
	}
	for _, forbidden := range []string{
		"agents",
		"claude",
		"codex",
		"host",
		"local",
		"force",
	} {
		if projectRelocateCmd.Flags().Lookup(forbidden) != nil {
			t.Fatalf("project relocate exposes unsafe flag --%s", forbidden)
		}
	}
}
