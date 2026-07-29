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
// creates schema objects or applies migrations. Explicit `haft init` and
// `haft project migrate` are the only CLI effects that upgrade the durable
// project database.
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
	migrationCommand := fmt.Sprintf(
		"haft project migrate --project-root %q --project-id %s",
		projectRoot,
		projectID,
	)
	return fmt.Errorf(
		"haft project database is not ready for %s: %w; run `%s` to apply the explicit host-free database upgrade, then retry; no migration was attempted by %s",
		operation,
		cause,
		migrationCommand,
		operation,
	)
}
