package cli

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

var fpfCmd = &cobra.Command{
	Use:   "fpf",
	Short: "Query source-native FPF publications",
	Long: `Query source-native FPF publication units without selecting a pattern.

Concern queries return compact source-derived candidates. Lookup resolves an
exact identifier first and otherwise returns candidates. Inspect is exact-only.
All three commands return the closed exact_hit | candidate_set | abstained JSON
union; candidate order never implies applicability or work order.

Examples:
	  haft fpf query "How should I frame this concern?" --json
	  haft fpf lookup "A.22.CGUS" --role pattern_body --json
	  haft fpf inspect "A.22.CGUS" --role pattern_body --json`,
}

var fpfQueryCmd = &cobra.Command{
	Use:   "query <concern>",
	Short: "Retrieve compact source-derived candidates for a concern",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runFPFQuery,
}

var fpfLookupCmd = &cobra.Command{
	Use:   "lookup <identifier>",
	Short: "Resolve one exact source unit or return compact candidates",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runFPFLookup,
}

var fpfInspectCmd = &cobra.Command{
	Use:   "inspect <identifier>",
	Short: "Inspect one exact source unit or abstain",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runFPFInspect,
}

var (
	fpfQueryEntityOfConcern           string
	fpfQueryKnownContext              []string
	fpfQueryIntendedUse               string
	fpfQueryMaxCandidatesPerRole      int
	fpfQueryMaxTotalCandidates        int
	fpfQueryMaxExcerptCharacters      int
	fpfQueryMaxRelationsPerCandidate  int
	fpfQueryJSON                      bool
	fpfLookupRoles                    []string
	fpfLookupMaxCandidatesPerRole     int
	fpfLookupMaxTotalCandidates       int
	fpfLookupMaxExcerptCharacters     int
	fpfLookupMaxRelationsPerCandidate int
	fpfLookupJSON                     bool
	fpfInspectRoles                   []string
	fpfInspectJSON                    bool
	fpfPublicationView                string
	fpfReplayRef                      string
)

type fpfQueryIndexVerificationCache struct {
	once      sync.Once
	operation func(*sql.DB) error
	err       error
}

func newFPFQueryIndexVerificationCache(
	operation func(*sql.DB) error,
) *fpfQueryIndexVerificationCache {
	return &fpfQueryIndexVerificationCache{operation: operation}
}

func (cache *fpfQueryIndexVerificationCache) Verify(db *sql.DB) error {
	if cache == nil || cache.operation == nil {
		return fmt.Errorf("FPF query index verifier is unavailable")
	}
	cache.once.Do(func() {
		cache.err = cache.operation(db)
	})
	return cache.err
}

var openFPFDBFunc = openFPFDB

var embeddedFPFQueryIndexVerification = newFPFQueryIndexVerificationCache(fpf.VerifySourceQueryIndexReadOnlyDB)

var verifyFPFDBFunc = embeddedFPFQueryIndexVerification.Verify

func init() {
	fpfCmd.PersistentFlags().StringVar(
		&fpfPublicationView,
		"view",
		"working",
		"Public response view: working, trace, or diagnostic",
	)
	fpfCmd.PersistentFlags().StringVar(
		&fpfReplayRef,
		"replay-ref",
		"",
		"Opaque trace_ref to replay against the same source snapshot and typed request",
	)
	fpfQueryCmd.Flags().StringVar(&fpfQueryEntityOfConcern, "entity-of-concern", "", "Optional entity whose concern is being queried")
	fpfQueryCmd.Flags().StringSliceVar(&fpfQueryKnownContext, "known-context", nil, "Known context distinction; repeat for multiple values")
	fpfQueryCmd.Flags().StringVar(&fpfQueryIntendedUse, "intended-use", "", "Optional receiving use for the retrieved source units")
	fpfQueryCmd.Flags().IntVar(&fpfQueryMaxCandidatesPerRole, "max-candidates-per-role", 0, "Maximum candidates returned for each source role")
	fpfQueryCmd.Flags().IntVar(&fpfQueryMaxTotalCandidates, "max-total-candidates", 0, "Maximum candidates returned across all source roles")
	fpfQueryCmd.Flags().IntVar(&fpfQueryMaxExcerptCharacters, "max-excerpt-characters", 0, "Maximum total source-text characters per candidate across excerpt and use cues")
	fpfQueryCmd.Flags().IntVar(&fpfQueryMaxRelationsPerCandidate, "max-relations-per-candidate", 0, "Maximum typed source relations returned in each candidate projection")
	fpfQueryCmd.Flags().BoolVar(&fpfQueryJSON, "json", false, "Emit compact JSON instead of indented JSON")

	fpfLookupCmd.Flags().StringSliceVar(&fpfLookupRoles, "role", nil, "Source role; repeat as needed")
	fpfLookupCmd.Flags().IntVar(&fpfLookupMaxCandidatesPerRole, "max-candidates-per-role", 0, "Maximum candidates returned for each source role")
	fpfLookupCmd.Flags().IntVar(&fpfLookupMaxTotalCandidates, "max-total-candidates", 0, "Maximum candidates returned across all source roles")
	fpfLookupCmd.Flags().IntVar(&fpfLookupMaxExcerptCharacters, "max-excerpt-characters", 0, "Maximum total source-text characters per candidate across excerpt and use cues")
	fpfLookupCmd.Flags().IntVar(&fpfLookupMaxRelationsPerCandidate, "max-relations-per-candidate", 0, "Maximum typed source relations returned in each candidate projection")
	fpfLookupCmd.Flags().BoolVar(&fpfLookupJSON, "json", false, "Emit compact JSON instead of indented JSON")

	fpfInspectCmd.Flags().StringSliceVar(&fpfInspectRoles, "role", nil, "Source role; repeat as needed")
	fpfInspectCmd.Flags().BoolVar(&fpfInspectJSON, "json", false, "Emit compact JSON instead of indented JSON")

	fpfCmd.AddCommand(fpfQueryCmd)
	fpfCmd.AddCommand(fpfLookupCmd)
	fpfCmd.AddCommand(fpfInspectCmd)
	rootCmd.AddCommand(fpfCmd)
}

func runFPFQuery(cmd *cobra.Command, args []string) error {
	request := fpf.ConcernQuery{
		Text:            strings.Join(args, " "),
		EntityOfConcern: fpfQueryEntityOfConcern,
		KnownContext:    append([]string(nil), fpfQueryKnownContext...),
		IntendedUse:     fpfQueryIntendedUse,
		ResponseBudget: fpf.ResponseBudget{
			MaxCandidatesPerRole:     fpfQueryMaxCandidatesPerRole,
			MaxTotalCandidates:       fpfQueryMaxTotalCandidates,
			MaxExcerptCharacters:     fpfQueryMaxExcerptCharacters,
			MaxRelationsPerCandidate: fpfQueryMaxRelationsPerCandidate,
		},
	}
	publicationRequest, err := fpfPublicationRequest()
	if err != nil {
		return err
	}
	return executeFPFQueryCommand(
		cmd,
		request,
		publicationRequest,
		fpfQueryJSON,
	)
}

func runFPFLookup(cmd *cobra.Command, args []string) error {
	request := fpf.LookupQuery{
		Identifier: strings.Join(args, " "),
		Roles:      sourceUnitRoles(fpfLookupRoles),
		ResponseBudget: fpf.ResponseBudget{
			MaxCandidatesPerRole:     fpfLookupMaxCandidatesPerRole,
			MaxTotalCandidates:       fpfLookupMaxTotalCandidates,
			MaxExcerptCharacters:     fpfLookupMaxExcerptCharacters,
			MaxRelationsPerCandidate: fpfLookupMaxRelationsPerCandidate,
		},
	}
	publicationRequest, err := fpfPublicationRequest()
	if err != nil {
		return err
	}
	return executeFPFQueryCommand(
		cmd,
		request,
		publicationRequest,
		fpfLookupJSON,
	)
}

func runFPFInspect(cmd *cobra.Command, args []string) error {
	request := fpf.InspectQuery{
		Identifier: strings.Join(args, " "),
		Roles:      sourceUnitRoles(fpfInspectRoles),
	}
	publicationRequest, err := fpfPublicationRequest()
	if err != nil {
		return err
	}
	return executeFPFQueryCommand(
		cmd,
		request,
		publicationRequest,
		fpfInspectJSON,
	)
}

func fpfPublicationRequest() (fpf.QueryPublicationRequest, error) {
	return fpf.NewQueryPublicationRequest(fpfPublicationView, fpfReplayRef)
}

func executeFPFQueryCommand(
	cmd *cobra.Command,
	request fpf.QueryRequest,
	publicationRequest fpf.QueryPublicationRequest,
	compact bool,
) error {
	if publicationRequest.View() == "" {
		return fmt.Errorf("FPF query publication request is required")
	}
	style := fpf.PublishedQueryJSONIndented
	if compact {
		style = fpf.PublishedQueryJSONCompact
	}
	payload, err := encodeEmbeddedFPFQuery(request, publicationRequest, style)
	if err != nil {
		return fmt.Errorf("FPF query: %w", err)
	}
	if _, err := cmd.OutOrStdout().Write(payload); err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write([]byte("\n"))
	return err
}

func encodeEmbeddedFPFQuery(
	request fpf.QueryRequest,
	publicationRequest fpf.QueryPublicationRequest,
	style fpf.PublishedQueryJSONStyle,
) ([]byte, error) {
	result, err := publishEmbeddedFPFQuery(request, publicationRequest)
	if err != nil {
		return nil, err
	}
	return fpf.EncodePublishedQuery(result, style)
}

func publishEmbeddedFPFQuery(
	request fpf.QueryRequest,
	publicationRequest fpf.QueryPublicationRequest,
) (fpf.PublishedQueryResult, error) {
	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if err := verifyFPFDBFunc(db); err != nil {
		return nil, fmt.Errorf("verify canonical FPF source index: %w", err)
	}
	snapshot, err := fpf.LoadQuerySourceSnapshot(db)
	if err != nil {
		return nil, fmt.Errorf("load canonical FPF source snapshot: %w", err)
	}
	preflight, err := fpf.NewQueryReplayPreflight(request, snapshot)
	if err != nil {
		return nil, err
	}
	mismatch, proceed, err := preflight.Check(publicationRequest)
	if err != nil {
		return nil, err
	}
	if !proceed {
		return mismatch, nil
	}
	index := fpf.NewSQLiteQueryIndex(db)
	evaluation, err := fpf.EvaluateQuery(index, request)
	if err != nil {
		return nil, err
	}
	execution, err := preflight.Complete(evaluation)
	if err != nil {
		return nil, err
	}
	return fpf.ProjectQueryResult(execution, publicationRequest)
}

func queryEmbeddedFPF(request fpf.QueryRequest) (fpf.QueryResult, error) {
	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if err := verifyFPFDBFunc(db); err != nil {
		return nil, fmt.Errorf("verify canonical FPF source index: %w", err)
	}
	index := fpf.NewSQLiteQueryIndex(db)
	return fpf.Query(index, request)
}

func sourceUnitRoles(values []string) []fpf.SourceUnitRole {
	roles := make([]fpf.SourceUnitRole, 0, len(values))
	for _, value := range values {
		roles = append(roles, fpf.SourceUnitRole(value))
	}
	return roles
}

func openFPFDB() (*sql.DB, func(), error) {
	return openFPFDBContext(context.Background())
}

func openFPFDBContext(ctx context.Context) (*sql.DB, func(), error) {
	return openFPFDBImage(ctx, embeddedFPFDB)
}

func openFPFDBImage(
	ctx context.Context,
	databaseImage []byte,
) (*sql.DB, func(), error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("open embedded FPF database: context is required")
	}
	if len(databaseImage) == 0 {
		return nil, nil, fmt.Errorf("open embedded FPF database: image is empty")
	}

	tmpDir, err := os.MkdirTemp("", "haft-fpf-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir: %w", err)
	}
	removeTempDir := func() {
		_ = os.RemoveAll(tmpDir)
	}

	dbPath := filepath.Join(tmpDir, "fpf.db")
	if err := os.WriteFile(dbPath, databaseImage, 0600); err != nil {
		removeTempDir()
		return nil, nil, fmt.Errorf("write temp db: %w", err)
	}
	if err := os.Chmod(dbPath, 0400); err != nil {
		removeTempDir()
		return nil, nil, fmt.Errorf("make temp db read-only: %w", err)
	}
	absolutePath, err := filepath.Abs(dbPath)
	if err != nil {
		removeTempDir()
		return nil, nil, fmt.Errorf("resolve temp db path: %w", err)
	}

	readOnlyURI := url.URL{Scheme: "file", Path: absolutePath}
	query := readOnlyURI.Query()
	query.Set("mode", "ro")
	query.Set("immutable", "1")
	query.Set("_pragma", "query_only(1)")
	readOnlyURI.RawQuery = query.Encode()

	db, err := sql.Open("sqlite", readOnlyURI.String())
	if err != nil {
		removeTempDir()
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		removeTempDir()
		return nil, nil, fmt.Errorf("ping read-only embedded FPF db: %w", err)
	}

	cleanup := func() {
		_ = db.Close()
		removeTempDir()
	}
	return db, cleanup, nil
}
