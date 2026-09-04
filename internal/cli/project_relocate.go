package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledgermigration"
	"github.com/spf13/cobra"
)

var (
	projectRelocateFromRoot string
	projectRelocateToRoot   string
	projectRelocateID       string
)

var projectRelocateCmd = &cobra.Command{
	Use:   "relocate",
	Short: "Attach a moved project directory to its existing ledger",
	Long: `Attach one moved Haft project directory to its existing project ledger.

The command requires the exact previous root, current root, and project ID. The
previous root must no longer exist. Before changing the attachment, Haft creates
and verifies a consistent SQLite backup. The immutable genesis binding and all
historical project-root records remain unchanged; the new root is appended to
the relocation lineage. No host configuration is changed.`,
	Args: cobra.NoArgs,
	RunE: runProjectRelocate,
}

func init() {
	projectRelocateCmd.Flags().StringVar(
		&projectRelocateFromRoot,
		"from-root",
		"",
		"Exact previous project root, which must no longer exist",
	)
	projectRelocateCmd.Flags().StringVar(
		&projectRelocateToRoot,
		"project-root",
		"",
		"Exact current project root containing .haft/project.yaml",
	)
	projectRelocateCmd.Flags().StringVar(
		&projectRelocateID,
		"project-id",
		"",
		"Exact expected project identity (qnt_........)",
	)
	projectCmd.AddCommand(projectRelocateCmd)
}

func runProjectRelocate(cmd *cobra.Command, _ []string) error {
	fromRoot := strings.TrimSpace(projectRelocateFromRoot)
	if fromRoot == "" {
		return fmt.Errorf(
			"--from-root is required; no project-root relocation was attempted",
		)
	}
	toRoot := strings.TrimSpace(projectRelocateToRoot)
	if toRoot == "" {
		return fmt.Errorf(
			"--project-root is required; no project-root relocation was attempted",
		)
	}
	projectID := strings.TrimSpace(projectRelocateID)
	if projectID == "" {
		return fmt.Errorf(
			"--project-id is required; no project-root relocation was attempted",
		)
	}
	request, err := projectledgermigration.NewRootRelocationRequest(
		fromRoot,
		toRoot,
		projectID,
	)
	if err != nil {
		return fmt.Errorf(
			"prepare project-root relocation: %w; no project-root relocation was attempted",
			err,
		)
	}
	result, err := projectledgermigration.RelocateRoot(
		cmd.Context(),
		request,
		time.Now().UTC(),
	)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(
		cmd.OutOrStdout(),
		"Project root relocation: %s\nProject ID: %s\nPrevious root: %s\nCurrent root: %s\nSchema: %d -> %d\nRelocation record: %d (%s)\nVerified backup: %s\nBackup digest: %s\nHost effects: none\nNext step: restart or reconnect the host so it opens the current root.\n",
		result.Outcome,
		result.ProjectID,
		result.FromRoot,
		result.ToRoot,
		result.BeforeSchema,
		result.AfterSchema,
		result.RelocationNumber,
		result.RelocationDigest,
		result.BackupPath,
		result.BackupDigest,
	)
	if err != nil {
		return fmt.Errorf("write project-root relocation result: %w", err)
	}
	return nil
}
