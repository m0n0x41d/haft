package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

// openCurrentProjectLedger is the ordinary-runtime ledger boundary. It never
// creates schema objects or applies migrations. `haft serve` owns its separate
// startup activation boundary before opening project-backed surfaces;
// explicit `haft init` and `haft project migrate` remain the non-server
// upgrade effects.
func openCurrentProjectLedger(
	ctx context.Context,
	projectRoot string,
	access projectledger.Access,
	operation string,
) (*projectledger.Handle, error) {
	canonicalRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return nil, currentProjectLedgerError(projectRoot, operation, err)
	}
	canonicalRoot = filepath.Clean(canonicalRoot)
	ledger, err := projectledger.OpenExisting(ctx, canonicalRoot, access)
	if err != nil {
		return nil, currentProjectLedgerError(canonicalRoot, operation, err)
	}
	if err := db.RequireCurrentSchemaReadOnly(ctx, ledger.Database()); err != nil {
		closeErr := ledger.Close()
		return nil, currentProjectLedgerError(
			canonicalRoot,
			operation,
			errors.Join(err, closeErr),
		)
	}
	return ledger, nil
}

func currentProjectLedgerError(
	projectRoot string,
	operation string,
	cause error,
) error {
	projectID := expectedProjectIDForRoot(projectRoot)
	if projectID == "" {
		projectID = "<project-id>"
	}
	repair := currentProjectLedgerRepair(projectRoot, projectID, cause)
	return fmt.Errorf(
		"haft project database is not ready for %s: %w; run `%s` to %s, then retry; no migration was attempted and no binding recovery was attempted by %s",
		operation,
		cause,
		repair.command,
		repair.effect,
		operation,
	)
}

type projectLedgerRepair struct {
	command string
	effect  string
}

func currentProjectLedgerRepair(
	projectRoot string,
	projectID string,
	cause error,
) projectLedgerRepair {
	if errors.Is(cause, projectledger.ErrBindingMissing) {
		command := fmt.Sprintf(
			"haft project recover-binding --project-root %q --project-id %s",
			projectRoot,
			projectID,
		)
		return projectLedgerRepair{
			command: command,
			effect:  "recover the exact missing durable binding from a consistent backup",
		}
	}
	command := fmt.Sprintf(
		"haft project migrate --project-root %q --project-id %s",
		projectRoot,
		projectID,
	)
	return projectLedgerRepair{
		command: command,
		effect:  "apply the explicit host-free database upgrade",
	}
}
