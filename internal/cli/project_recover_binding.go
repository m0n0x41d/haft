package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledgermigration"
	"github.com/spf13/cobra"
)

var (
	projectRecoverBindingRoot string
	projectRecoverBindingID   string
)

var projectRecoverBindingCmd = &cobra.Command{
	Use:   "recover-binding",
	Short: "Recover one exact binding-aware ledger with a missing identity binding",
	Long: `Recover the missing immutable project identity binding in one exact
binding-aware Haft ledger after creating and verifying a consistent SQLite
backup. This command rejects pre-binding or newer schemas, existing bindings, identity
mismatches, integrity failures, and foreign-key violations. It does not publish
or modify host configuration, skills, instructions, hooks, or project carriers.`,
	Args: cobra.NoArgs,
	RunE: runProjectRecoverBinding,
}

func init() {
	projectRecoverBindingCmd.Flags().StringVar(
		&projectRecoverBindingRoot,
		"project-root",
		"",
		"Exact project root containing .haft/project.yaml",
	)
	projectRecoverBindingCmd.Flags().StringVar(
		&projectRecoverBindingID,
		"project-id",
		"",
		"Exact expected project identity (qnt_........)",
	)
	projectCmd.AddCommand(projectRecoverBindingCmd)
}

func runProjectRecoverBinding(cmd *cobra.Command, _ []string) error {
	root := strings.TrimSpace(projectRecoverBindingRoot)
	if root == "" {
		return fmt.Errorf(
			"--project-root is required; no binding recovery was attempted",
		)
	}
	projectID := strings.TrimSpace(projectRecoverBindingID)
	if projectID == "" {
		return fmt.Errorf(
			"--project-id is required; no binding recovery was attempted",
		)
	}
	request, err := projectledgermigration.NewRequest(root, projectID)
	if err != nil {
		return fmt.Errorf(
			"prepare exact project binding recovery: %w; no binding recovery was attempted",
			err,
		)
	}
	result, err := projectledgermigration.RecoverMissingBinding(
		cmd.Context(),
		request,
		time.Now().UTC(),
	)
	if err != nil {
		return err
	}
	output := cmd.OutOrStdout()
	_, err = fmt.Fprintf(
		output,
		"Project binding recovery: %s\nProject ID: %s\nProject root: %s\nSchema: %d\nDatabase: %s\nBackup: %s\nBackup digest: %s\nHost effects: none\n",
		result.Outcome,
		result.ProjectID,
		result.ProjectRoot,
		result.SchemaVersion,
		result.DatabasePath,
		result.BackupPath,
		result.BackupDigest,
	)
	if err != nil {
		return fmt.Errorf(
			"write project binding recovery result: %w",
			err,
		)
	}
	return nil
}
