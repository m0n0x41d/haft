package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/spf13/cobra"
)

func TestProjectRecoverBindingCommandRequiresExactRootAndIdentity(
	t *testing.T,
) {
	previousRoot := projectRecoverBindingRoot
	previousID := projectRecoverBindingID
	projectRecoverBindingRoot = ""
	projectRecoverBindingID = ""
	t.Cleanup(func() {
		projectRecoverBindingRoot = previousRoot
		projectRecoverBindingID = previousID
	})

	command := &cobra.Command{}
	command.SetOut(&bytes.Buffer{})
	err := runProjectRecoverBinding(command, nil)
	if err == nil || !strings.Contains(err.Error(), "--project-root is required") {
		t.Fatalf("missing-root error = %v", err)
	}

	projectRecoverBindingRoot = t.TempDir()
	err = runProjectRecoverBinding(command, nil)
	if err == nil || !strings.Contains(err.Error(), "--project-id is required") {
		t.Fatalf("missing-id error = %v", err)
	}
}

func TestProjectRecoverBindingCommandExposesOnlyExactIdentityFlags(
	t *testing.T,
) {
	t.Parallel()

	if projectRecoverBindingCmd.Flags().Lookup("project-root") == nil {
		t.Fatal("project recover-binding is missing --project-root")
	}
	if projectRecoverBindingCmd.Flags().Lookup("project-id") == nil {
		t.Fatal("project recover-binding is missing --project-id")
	}
	for _, forbidden := range []string{
		"agents",
		"claude",
		"codex",
		"host",
		"local",
	} {
		if projectRecoverBindingCmd.Flags().Lookup(forbidden) != nil {
			t.Fatalf(
				"project recover-binding exposes host flag --%s",
				forbidden,
			)
		}
	}
}

func TestCurrentProjectLedgerRepairRoutesMissingBindingToRecovery(
	t *testing.T,
) {
	t.Parallel()

	repair := currentProjectLedgerRepair(
		"/project",
		"qnt_12345678",
		errors.Join(
			projectledger.ErrBindingMissing,
			errors.New("wrapped"),
		),
	)
	if !strings.Contains(repair.command, "project recover-binding") {
		t.Fatalf("missing-binding repair command = %q", repair.command)
	}
	if strings.Contains(repair.command, "project migrate") {
		t.Fatalf("missing-binding repair routes to migration: %q", repair.command)
	}

	repair = currentProjectLedgerRepair(
		"/current/project",
		"qnt_12345678",
		&projectledger.BindingRootMismatchError{
			StoredProjectID:    "qnt_12345678",
			StoredRoot:         "/previous/project",
			RequestedProjectID: "qnt_12345678",
			RequestedRoot:      "/current/project",
		},
	)
	for _, fragment := range []string{
		"project relocate",
		`--from-root "/previous/project"`,
		`--project-root "/current/project"`,
		"--project-id qnt_12345678",
	} {
		if !strings.Contains(repair.command, fragment) {
			t.Fatalf(
				"root-mismatch repair command missing %q: %s",
				fragment,
				repair.command,
			)
		}
	}
	if strings.Contains(repair.command, "recover-binding") {
		t.Fatalf("root mismatch routes to binding recovery: %q", repair.command)
	}

	repair = currentProjectLedgerRepair(
		"/project",
		"qnt_12345678",
		errors.New("schema is old"),
	)
	if !strings.Contains(repair.command, "project migrate") {
		t.Fatalf("old-schema repair command = %q", repair.command)
	}
}
