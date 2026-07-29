package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	methodpkg "github.com/m0n0x41d/haft/internal/method"
)

var (
	methodCatalogJSON    bool
	methodCatalogStatus  string
	methodCatalogScopeID string
)

var methodCmd = &cobra.Command{
	Use:   "method",
	Short: "Inspect Haft MethodPack catalog state",
}

var methodCatalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "Read-only MethodPack lifecycle catalog",
	Long: `Print the MethodPack catalog as a read-only discovery view.

The catalog is not a ProcessPattern object, not an enforcement authority, and
not proof of gate passage. Current methods are eligible for task-local pull
matching; superseded and deprecated methods remain history/detail only.`,
	RunE: runMethodCatalog,
}

func runMethodCatalog(cmd *cobra.Command, args []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	result, _, err := handleHaftMethodForProject(
		ctx,
		nil,
		filepath.Join(projectRoot, ".haft"),
		map[string]any{
			"action":        "catalog",
			"method_status": methodCatalogStatus,
			"scope_id":      methodCatalogScopeID,
		},
	)
	if err != nil {
		return err
	}
	if methodCatalogJSON {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), result)
		return err
	}
	applicability := methodProfileApplicabilityResponse{}
	if err := json.Unmarshal([]byte(result), &applicability); err == nil &&
		applicability.Kind == "haft_method_profile_applicability" {
		return writeMethodCatalogApplicabilityText(
			cmd.OutOrStdout(),
			applicability,
		)
	}
	report := methodpkg.CatalogReport{}
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		return err
	}
	return writeMethodCatalogText(cmd.OutOrStdout(), report)
}

func writeMethodCatalogApplicabilityText(
	w io.Writer,
	response methodProfileApplicabilityResponse,
) error {
	if _, err := fmt.Fprintf(
		w,
		"Haft MethodPack catalog applicability=%s scope_id=%s\n",
		response.Applicability,
		response.ScopeID,
	); err != nil {
		return err
	}
	if response.ProfileApplicability.Cue != nil {
		if _, err := fmt.Fprintln(
			w,
			response.ProfileApplicability.Cue.Message,
		); err != nil {
			return err
		}
	}
	for _, boundary := range response.Boundary {
		if _, err := fmt.Fprintf(w, "- %s\n", boundary); err != nil {
			return err
		}
	}
	return nil
}

func writeMethodCatalogText(w io.Writer, report methodpkg.CatalogReport) error {
	if _, err := fmt.Fprintf(w, "Haft MethodPack catalog %s@%s\n", report.CatalogID, report.CatalogVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "status=%s returned=%d total=%d authority=%s\n", report.FilterStatus, report.Summary.Returned, report.Summary.Total, report.AuthorityBoundary); err != nil {
		return err
	}
	for _, entry := range report.Methods {
		if _, err := fmt.Fprintf(w, "- %s `%s` lifecycle=%s\n", entry.Title, entry.ID, entry.Lifecycle.Status); err != nil {
			return err
		}
		if entry.FirstUsefulMove != "" {
			if _, err := fmt.Fprintf(w, "  first_useful_move: %s\n", entry.FirstUsefulMove); err != nil {
				return err
			}
		}
		if len(entry.SourcePatternRefs) > 0 {
			if _, err := fmt.Fprintf(w, "  source_pattern_refs: %v (documentary context only, not gate evidence)\n", entry.SourcePatternRefs); err != nil {
				return err
			}
		}
	}
	return nil
}

func init() {
	methodCatalogCmd.Flags().BoolVar(&methodCatalogJSON, "json", false, "print structured JSON output")
	methodCatalogCmd.Flags().StringVar(&methodCatalogStatus, "status", methodpkg.LifecycleCurrent, "method lifecycle status: current, experimental, superseded, deprecated, all")
	methodCatalogCmd.Flags().StringVar(&methodCatalogScopeID, "scope-id", "", "Exact canonical project ScopeID for a mixed admitted profile")
	methodCmd.AddCommand(methodCatalogCmd)
	rootCmd.AddCommand(methodCmd)
}
