package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codeintel"
)

var (
	graphExploreSymbol        string
	graphExploreQuery         string
	graphExploreFile          string
	graphExploreLine          int
	graphExploreMaxCandidates int
	graphExploreView          string
	graphExploreTraceRef      string
	graphExploreJSON          bool
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Read the fused code and reasoning graph",
}

var graphExploreCmd = &cobra.Command{
	Use:   "explore",
	Short: "Explore one exact symbol or bounded concern",
	Long: `Explore one exact symbol or one bounded natural-language concern.

The command and haft_query(action="explore") share one canonical execution,
projection, and JSON encoder. working is the default bounded view; trace adds
replay basis; diagnostic adds retrieval and traversal internals.`,
	RunE: runGraphExplore,
}

func init() {
	graphExploreCmd.Flags().StringVar(
		&graphExploreSymbol,
		"symbol",
		"",
		"exact symbol name or a comma/space-separated symbol bag",
	)
	graphExploreCmd.Flags().StringVar(
		&graphExploreQuery,
		"query",
		"",
		"bounded concern query",
	)
	graphExploreCmd.Flags().StringVar(
		&graphExploreFile,
		"file",
		"",
		"optional file filter for an exact symbol",
	)
	graphExploreCmd.Flags().IntVar(
		&graphExploreLine,
		"line",
		0,
		"optional 1-based line filter for an exact symbol",
	)
	graphExploreCmd.Flags().IntVar(
		&graphExploreMaxCandidates,
		"max-candidates",
		codeintel.DefaultConcernCandidateBudget,
		"concern candidate retrieval budget",
	)
	graphExploreCmd.Flags().StringVar(
		&graphExploreView,
		"view",
		"working",
		"publication view: working, trace, or diagnostic",
	)
	graphExploreCmd.Flags().StringVar(
		&graphExploreTraceRef,
		"trace-ref",
		"",
		"opaque trace reference for replay comparison",
	)
	graphExploreCmd.Flags().BoolVar(
		&graphExploreJSON,
		"json",
		false,
		"emit the canonical JSON payload (the command is JSON-only in v9)",
	)
	graphCmd.AddCommand(graphExploreCmd)
	rootCmd.AddCommand(graphCmd)
}

func runGraphExplore(
	cmd *cobra.Command,
	_ []string,
) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}
	input, err := absProjectRootInput(projectRoot, "graph")
	if err != nil {
		return err
	}
	binding, err := resolveProjectBindingFromInput(input, "")
	if err != nil {
		return err
	}
	ledger, err := openServeProjectLedger(cmd.Context(), binding)
	if err != nil {
		return err
	}
	defer ledger.Close()
	store := artifact.NewStore(ledger.Database())
	coordinator, err := codeintel.NewProjectIndexCoordinator(
		codeintel.ProjectIndexCoordinates{
			ProjectID:   ledger.ProjectID().String(),
			ProjectRoot: ledger.ProjectRoot().String(),
			LedgerPath:  ledger.DatabasePath(),
		},
	)
	if err != nil {
		return err
	}
	codeIntelService := codeintel.NewServiceWithIndexCoordinator(
		store,
		coordinator,
	)

	wire, err := publishCodeExploreWithService(
		context.Background(),
		store,
		codeIntelService,
		projectRoot,
		graphExploreSymbol,
		graphExploreFile,
		graphExploreLine,
		graphExploreQuery,
		graphExploreMaxCandidates,
		graphExploreView,
		graphExploreTraceRef,
	)
	if err != nil {
		return err
	}
	return writeExactBytes(cmd.OutOrStdout(), wire)
}

func publishCodeExploreWithService(
	ctx context.Context,
	store *artifact.Store,
	service *codeintel.Service,
	projectRoot string,
	symbol string,
	file string,
	line int,
	query string,
	maxCandidates int,
	view string,
	traceRef string,
) ([]byte, error) {
	request, err := codeintel.NewExploreExecutionRequest(
		symbol,
		file,
		line,
		query,
		maxCandidates,
	)
	if err != nil {
		return nil, err
	}
	publication, err := codeintel.NewExplorePublicationRequest(
		view,
		traceRef,
	)
	if err != nil {
		return nil, err
	}
	result, err := codeintel.PublishExplore(
		ctx,
		service,
		projectRoot,
		request,
		publication,
	)
	if err != nil {
		return nil, err
	}
	return codeintel.EncodePublishedExplore(
		result,
		codeintel.PublishedExploreJSONCompact,
	)
}

func writeExactBytes(
	writer io.Writer,
	value []byte,
) error {
	written, err := writer.Write(value)
	if err != nil {
		return err
	}
	if written != len(value) {
		return io.ErrShortWrite
	}
	return nil
}

func stringExploreView(args map[string]any) string {
	view, _ := args["view"].(string)
	return strings.TrimSpace(view)
}

func stringExploreTraceRef(args map[string]any) string {
	traceRef, _ := args["trace_ref"].(string)
	return strings.TrimSpace(traceRef)
}
