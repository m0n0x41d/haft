package cli

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectledgermigration"
	"github.com/spf13/cobra"
)

var (
	projectMigrateRoot string
	projectMigrateID   string
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage exact project-bound core state",
}

var projectMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply additive core database migrations without host publication",
	Long: `Apply pending additive migrations to one already initialized and
identity-bound Haft project ledger.

This command never discovers or configures an agent host and never installs,
updates, or removes skills, instructions, MCP configuration, hooks, or host
packages. Both the project root and expected project identity are required.`,
	Args: cobra.NoArgs,
	RunE: runProjectMigrate,
}

func init() {
	projectMigrateCmd.Flags().StringVar(
		&projectMigrateRoot,
		"project-root",
		"",
		"Exact project root containing .haft/project.yaml",
	)
	projectMigrateCmd.Flags().StringVar(
		&projectMigrateID,
		"project-id",
		"",
		"Exact expected project identity (qnt_........)",
	)
	projectCmd.AddCommand(projectMigrateCmd)
	rootCmd.AddCommand(projectCmd)
}

func runProjectMigrate(cmd *cobra.Command, _ []string) error {
	root := strings.TrimSpace(projectMigrateRoot)
	if root == "" {
		return fmt.Errorf("--project-root is required; no migration was attempted")
	}
	projectID := strings.TrimSpace(projectMigrateID)
	if projectID == "" {
		return fmt.Errorf("--project-id is required; no migration was attempted")
	}
	request, err := projectledgermigration.NewRequest(root, projectID)
	if err != nil {
		return fmt.Errorf("prepare exact core migration: %w; no migration was attempted", err)
	}
	result, err := projectledgermigration.Apply(cmd.Context(), request)
	if err != nil {
		return err
	}
	output := cmd.OutOrStdout()
	if _, err := fmt.Fprintf(
		output,
		"Project core migration: %s\nProject ID: %s\nProject root: %s\nSchema: %d -> %d\nHost effects: none\n",
		result.Outcome,
		result.ProjectID,
		result.ProjectRoot,
		result.BeforeSchema,
		result.AfterSchema,
	); err != nil {
		return fmt.Errorf("write project migration result: %w", err)
	}
	if result.BackupPath != "" {
		if _, err := fmt.Fprintf(
			output,
			"Verified backup: %s\nBackup digest: %s\n",
			result.BackupPath,
			result.BackupDigest,
		); err != nil {
			return fmt.Errorf("write project migration backup result: %w", err)
		}
	}
	return nil
}
