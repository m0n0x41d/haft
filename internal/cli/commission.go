package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

var (
	commissionJSONPath string
	commissionRunnerID string
)

var commissionCmd = &cobra.Command{
	Use:   "commission",
	Short: "Manage WorkCommissions for execution harnesses",
	Long: `Manage WorkCommissions for execution harnesses.

WorkCommission is the bounded authorization boundary between a DecisionRecord
and a runtime such as Open-Sleigh.`,
}

var commissionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a WorkCommission from a JSON payload",
	RunE:  runCommissionCreate,
}

var commissionListRunnableCmd = &cobra.Command{
	Use:   "list-runnable",
	Short: "List queued or ready WorkCommissions",
	RunE:  runCommissionListRunnable,
}

var commissionClaimCmd = &cobra.Command{
	Use:   "claim <commission-id>",
	Short: "Claim a WorkCommission for preflight",
	Args:  cobra.ExactArgs(1),
	RunE:  runCommissionClaim,
}

func init() {
	commissionCreateCmd.Flags().StringVar(&commissionJSONPath, "json", "", "JSON payload path, or '-' for stdin")
	commissionClaimCmd.Flags().StringVar(&commissionRunnerID, "runner", "haft-cli", "runner id for the lease")

	commissionCmd.AddCommand(commissionCreateCmd)
	commissionCmd.AddCommand(commissionListRunnableCmd)
	commissionCmd.AddCommand(commissionClaimCmd)
	rootCmd.AddCommand(commissionCmd)
}

func runCommissionCreate(cmd *cobra.Command, _ []string) error {
	payload, err := readCommissionJSONPayload(cmd.InOrStdin(), commissionJSONPath)
	if err != nil {
		return err
	}

	args := map[string]any{
		"action":     "create",
		"commission": payload,
	}

	return withCommissionStore(func(ctx context.Context, store *artifact.Store) error {
		result, err := handleHaftCommission(ctx, store, args)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionListRunnable(cmd *cobra.Command, _ []string) error {
	args := map[string]any{
		"action": "list_runnable",
	}

	return withCommissionStore(func(ctx context.Context, store *artifact.Store) error {
		result, err := handleHaftCommission(ctx, store, args)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionClaim(cmd *cobra.Command, args []string) error {
	params := map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": args[0],
		"runner_id":     commissionRunnerID,
	}

	return withCommissionStore(func(ctx context.Context, store *artifact.Store) error {
		result, err := handleHaftCommission(ctx, store, params)
		return writeCommissionResult(cmd, result, err)
	})
}

func withCommissionStore(fn func(context.Context, *artifact.Store) error) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	haftDir := filepath.Join(projectRoot, ".haft")
	projCfg, err := project.Load(haftDir)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if projCfg == nil {
		return fmt.Errorf("project not initialized — run 'haft init' first")
	}

	dbPath, err := projCfg.DBPath()
	if err != nil {
		return fmt.Errorf("get DB path: %w", err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}
	defer database.Close()

	store := artifact.NewStore(database.GetRawDB())
	return fn(context.Background(), store)
}

func readCommissionJSONPayload(stdin io.Reader, path string) (map[string]any, error) {
	if path == "" {
		return nil, fmt.Errorf("--json is required")
	}

	data, err := readCommissionJSONBytes(stdin, path)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse commission JSON: %w", err)
	}

	return unwrapCommissionPayload(payload), nil
}

func readCommissionJSONBytes(stdin io.Reader, path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return data, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

func unwrapCommissionPayload(payload map[string]any) map[string]any {
	commission, ok := payload["commission"].(map[string]any)
	if ok {
		return commission
	}
	return payload
}

func writeCommissionResult(cmd *cobra.Command, result string, err error) error {
	if err != nil {
		return err
	}

	_, writeErr := fmt.Fprintln(cmd.OutOrStdout(), result)
	return writeErr
}
