package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/fpfrefresh"
)

const refreshToolTemporaryPrefix = "haft-fpf-refresh-tool-"

var checkCandidate = fpfrefresh.CheckCandidate

func main() {
	code, err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fpf-refresh: %v\n", err)
	}
	if code != 0 {
		os.Exit(code)
	}
}

func run(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) (int, error) {
	if len(args) == 0 {
		return 2, fmt.Errorf(
			"usage: fpf-refresh <check|apply|rebase-local-practice|resume|restore|verify> [options]",
		)
	}
	switch args[0] {
	case "check":
		return runCheck(ctx, args[1:], stdout, stderr, false)
	case "apply":
		return runCheck(ctx, args[1:], stdout, stderr, true)
	case "rebase-local-practice":
		return runLocalPracticeRebase(args[1:], stdout, stderr)
	case "resume":
		return runRecovery(ctx, args[1:], stdout, false)
	case "restore":
		return runRecovery(ctx, args[1:], stdout, true)
	case "verify":
		return runVerify(ctx, args[1:], stdout)
	default:
		return 2, fmt.Errorf("unknown command %q", args[0])
	}
}

type checkFlags struct {
	root         string
	candidateRef string
	noFetch      bool
	reportPath   string
}

func runCheck(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	apply bool,
) (exitCode int, resultErr error) {
	commandName := "check"
	if apply {
		commandName = "apply"
	}
	options := checkFlags{}
	flags := flag.NewFlagSet(commandName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.root, "repo", ".", "repository root")
	flags.StringVar(
		&options.candidateRef,
		"candidate-ref",
		fpfrefresh.DefaultCandidateRef,
		"exact commit or ref to resolve once",
	)
	flags.BoolVar(
		&options.noFetch,
		"no-fetch",
		false,
		"do not fetch before resolving the candidate ref",
	)
	flags.StringVar(
		&options.reportPath,
		"report",
		"",
		"canonical report path (default .context/fpf-refresh/latest-report.json; '-' disables writing)",
	)
	if apply {
		flags.Bool(
			"accept-review-ready",
			false,
			"deprecated compatibility flag; review-ready source-current candidates are applied by default and this grants no release authority",
		)
	}
	if err := flags.Parse(args); err != nil {
		return 2, err
	}
	if flags.NArg() != 0 {
		return 2, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	layout, err := fpfrefresh.ResolveRepositoryLayout(options.root)
	if err != nil {
		return 1, err
	}
	if options.reportPath != "" && options.reportPath != "-" {
		reportPath, err := filepath.Abs(filepath.Clean(options.reportPath))
		if err != nil {
			return 1, err
		}
		layout.Report = reportPath
	}
	if options.reportPath != "-" {
		if err := fpfrefresh.ValidateReportPath(layout, layout.Report); err != nil {
			return 1, err
		}
	}
	tokenGate, err := fpfrefresh.ReadTokenGateCoordinates(layout.TokenGateFixture)
	if err != nil {
		return 1, err
	}
	toolRevision, err := exactRefreshToolRevision(ctx, layout.Root)
	if err != nil {
		return 1, err
	}
	indexerPath, cleanupIndexer, err := buildIndexerExecutable(ctx, layout.Root)
	if err != nil {
		return 1, err
	}
	defer func() {
		exitCode, resultErr = joinCleanupResult(
			exitCode,
			resultErr,
			"clean temporary indexer build",
			cleanupIndexer(),
		)
	}()
	if err := verifyRefreshToolRevision(ctx, layout.Root, toolRevision); err != nil {
		return 1, err
	}
	var fetch *fpfrefresh.GitFetchRequest
	if !options.noFetch {
		fetch = &fpfrefresh.GitFetchRequest{Remote: "origin"}
	}
	check, checkErr := checkCandidate(ctx, fpfrefresh.CandidateCheckRequest{
		Layout:       layout,
		CandidateRef: options.candidateRef,
		Fetch:        fetch,
		Builder: fpfrefresh.ExecutableIndexBuilder{
			ExecutablePath:       indexerPath,
			SourceRepositoryPath: layout.SourceRepository,
		},
		ToolRevision: toolRevision,
		TokenGate:    &tokenGate,
		TokenGateVerifier: fpfrefresh.ExecutableCandidateTokenGate{
			ShellPath:  "/bin/bash",
			ScriptPath: filepath.Join(layout.Root, "scripts", "fpf_query_token_gate.sh"),
		},
	})
	defer func() {
		exitCode, resultErr = joinCleanupResult(
			exitCode,
			resultErr,
			"clean checked candidate artifact",
			check.Cleanup(),
		)
	}()
	if checkErr != nil && len(check.Report.CanonicalBytes()) == 0 {
		return 1, checkErr
	}
	if err := check.Report.Verify(); err != nil {
		return 1, errors.Join(checkErr, fmt.Errorf("verify candidate check report: %w", err))
	}
	postCheckToolErr := verifyRefreshToolRevision(ctx, layout.Root, toolRevision)

	if _, err := fmt.Fprint(stdout, check.Report.Readable()); err != nil {
		return 1, errors.Join(checkErr, postCheckToolErr, err)
	}
	if err := writeExecutionTimings(stdout, check.ExecutionTimings); err != nil {
		return 1, errors.Join(checkErr, postCheckToolErr, err)
	}
	if options.reportPath != "-" {
		if err := fpfrefresh.ValidateReportPath(layout, layout.Report); err != nil {
			return 1, errors.Join(checkErr, postCheckToolErr, err)
		}
		if err := fpfrefresh.WriteCompatibilityReport(layout.Report, check.Report); err != nil {
			return 1, errors.Join(checkErr, postCheckToolErr, err)
		}
		if _, err := fmt.Fprintf(stdout, "report: %s\n", layout.Report); err != nil {
			return 1, errors.Join(checkErr, postCheckToolErr, err)
		}
	}
	if checkErr != nil || postCheckToolErr != nil {
		return 1, errors.Join(checkErr, postCheckToolErr)
	}
	state := check.Report.Outcome().State()
	if !apply {
		if err := writeNextCommand(
			stdout,
			layout.Root,
			state,
			check.CandidateSource.CommitSHA(),
		); err != nil {
			return 1, err
		}
		if state == fpfrefresh.StateCandidateRejected {
			return 1, nil
		}
		return 0, nil
	}

	switch state {
	case fpfrefresh.StateNoChange:
		if err := rebaseLiveLocalPractice(layout, stdout); err != nil {
			return 1, err
		}
		if _, err := fmt.Fprintln(
			stdout,
			"apply: no change; live source/database publication was not mutated",
		); err != nil {
			return 1, err
		}
		return 0, nil
	case fpfrefresh.StateCandidateRejected:
		return 1, fmt.Errorf("candidate is rejected; apply did not run")
	case fpfrefresh.StateReviewReady:
	case fpfrefresh.StateApplyReady:
	default:
		return 1, fmt.Errorf("unsupported check state %s", state.String())
	}
	if err := verifyRefreshToolRevision(ctx, layout.Root, toolRevision); err != nil {
		return 1, err
	}
	result, err := fpfrefresh.ApplyCheckedCandidate(
		ctx,
		fpfrefresh.ApplyCandidateRequest{
			Layout:                         layout,
			Check:                          check,
			AllowReviewReadyTechnicalApply: state == fpfrefresh.StateReviewReady,
		},
	)
	if err != nil {
		return 1, err
	}
	if _, err := fmt.Fprintf(
		stdout,
		"apply: complete; exact receipt archived at %s\n",
		result.ReceiptArchivePath,
	); err != nil {
		return 1, err
	}
	if _, err := fmt.Fprintln(
		stdout,
		"authority: no commit, semantic binding, SpecSection lifecycle act, ProjectTypeEnvHead activation, install, restart, P13, P14, push, tag, or release was performed",
	); err != nil {
		return 1, err
	}
	if err := rebaseLiveLocalPractice(layout, stdout); err != nil {
		return 1, err
	}
	return 0, nil
}

func runLocalPracticeRebase(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) (int, error) {
	flags := flag.NewFlagSet("rebase-local-practice", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := "."
	flags.StringVar(&root, "repo", ".", "repository root")
	if err := flags.Parse(args); err != nil {
		return 2, err
	}
	if flags.NArg() != 0 {
		return 2, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	layout, err := fpfrefresh.ResolveRepositoryLayout(root)
	if err != nil {
		return 1, err
	}
	if err := rebaseLiveLocalPractice(layout, stdout); err != nil {
		return 1, err
	}
	return 0, nil
}

func rebaseLiveLocalPractice(
	layout fpfrefresh.RepositoryLayout,
	stdout io.Writer,
) error {
	payload, err := os.ReadFile(layout.IntegrationLock)
	if err != nil {
		return fmt.Errorf("read generated FPF integration lock: %w", err)
	}
	lock, err := fpfrefresh.ParseIntegrationLock(payload)
	if err != nil {
		return err
	}
	result, err := fpfrefresh.RebaseLocalPracticeCandidate(
		layout.LatestLocalPracticeCandidate,
		layout.Database,
		lock.Coordinates.SourceRevision,
		lock.Coordinates.SpecDocumentDigest,
	)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(
		stdout,
		"local-practice: current=%t base=%s fpf_spec_pins=%d\n",
		!result.Changed,
		result.BaseTypeEnvRef,
		result.SourcePinCount,
	)
	return err
}

func joinCleanupResult(
	exitCode int,
	resultErr error,
	cleanupAction string,
	cleanupErr error,
) (int, error) {
	if cleanupErr == nil {
		return exitCode, resultErr
	}
	if exitCode == 0 {
		exitCode = 1
	}
	return exitCode, joinCleanupError(resultErr, cleanupAction, cleanupErr)
}

func joinCleanupError(
	resultErr error,
	cleanupAction string,
	cleanupErr error,
) error {
	if cleanupErr == nil {
		return resultErr
	}
	return errors.Join(
		resultErr,
		fmt.Errorf("%s: %w", cleanupAction, cleanupErr),
	)
}

func runRecovery(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	restore bool,
) (int, error) {
	command := "resume"
	if restore {
		command = "restore"
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	root := flags.String("repo", ".", "repository root")
	if err := flags.Parse(args); err != nil {
		return 2, err
	}
	if flags.NArg() != 0 {
		return 2, fmt.Errorf("unexpected positional arguments")
	}
	layout, err := fpfrefresh.ResolveRepositoryLayout(*root)
	if err != nil {
		return 1, err
	}
	var receipt fpfrefresh.ApplyReceipt
	if restore {
		receipt, err = fpfrefresh.ExecuteReceiptRestore(ctx, layout.Receipt)
	} else {
		receipt, err = fpfrefresh.ExecuteReceiptResume(ctx, layout.Receipt)
	}
	if err != nil {
		return 1, err
	}
	archivePath, err := fpfrefresh.ArchiveTerminalReceipt(
		layout.Receipt,
		filepath.Join(layout.StateDirectory, "receipts"),
	)
	if err != nil {
		return 1, err
	}
	if _, err := fmt.Fprintf(
		stdout,
		"%s: %s; receipt archived at %s\n",
		command,
		receipt.State,
		archivePath,
	); err != nil {
		return 1, err
	}
	return 0, nil
}

func runVerify(
	ctx context.Context,
	args []string,
	stdout io.Writer,
) (int, error) {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	root := flags.String("repo", ".", "repository root")
	if err := flags.Parse(args); err != nil {
		return 2, err
	}
	if flags.NArg() != 0 {
		return 2, fmt.Errorf("unexpected positional arguments")
	}
	layout, err := fpfrefresh.ResolveRepositoryLayout(*root)
	if err != nil {
		return 1, err
	}
	tokenGate, err := fpfrefresh.ReadTokenGateCoordinates(layout.TokenGateFixture)
	if err != nil {
		return 1, err
	}
	toolRevision, err := exactRefreshToolRevision(ctx, layout.Root)
	if err != nil {
		return 1, err
	}
	err = fpfrefresh.VerifyRepositoryIntegration(
		ctx,
		layout,
		toolRevision,
		&tokenGate,
		func(lock fpfrefresh.IntegrationLock) error {
			if err := verifyRefreshToolRevision(ctx, layout.Root, toolRevision); err != nil {
				return err
			}
			_, err := fmt.Fprintf(
				stdout,
				"fpf integration OK: source=%s db=%s units=%d typeenv=%s compiler=%s\n",
				lock.Coordinates.SourceRevision,
				lock.Coordinates.DatabaseDigest,
				lock.Coordinates.SourceUnitCount,
				lock.Coordinates.BaseTypeEnvRef,
				lock.Coordinates.TypeEnvCompilerEdition,
			)
			return err
		},
	)
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func buildIndexerExecutable(
	ctx context.Context,
	root string,
) (string, func() error, error) {
	return buildIndexerExecutableWithOperations(
		ctx,
		root,
		indexerBuildOperations{
			mkdirTemp: os.MkdirTemp,
			removeAll: os.RemoveAll,
			build: func(ctx context.Context, root string, path string) ([]byte, error) {
				command := exec.CommandContext(
					ctx,
					"go",
					"build",
					"-trimpath",
					"-o",
					path,
					"./cmd/indexer",
				)
				command.Dir = root
				return command.CombinedOutput()
			},
		},
	)
}

type indexerBuildOperations struct {
	mkdirTemp func(string, string) (string, error)
	removeAll func(string) error
	build     func(context.Context, string, string) ([]byte, error)
}

func buildIndexerExecutableWithOperations(
	ctx context.Context,
	root string,
	operations indexerBuildOperations,
) (string, func() error, error) {
	if operations.mkdirTemp == nil || operations.removeAll == nil || operations.build == nil {
		return "", nil, fmt.Errorf("indexer build operations are incomplete")
	}
	directory, err := operations.mkdirTemp("", refreshToolTemporaryPrefix)
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error {
		if !strings.HasPrefix(filepath.Base(directory), refreshToolTemporaryPrefix) {
			return fmt.Errorf("refuse to clean unexpected indexer build directory %q", directory)
		}
		return operations.removeAll(directory)
	}
	path := filepath.Join(directory, "haft-fpf-indexer")
	output, err := operations.build(ctx, root, path)
	if err != nil {
		buildErr := fmt.Errorf(
			"build candidate indexer: %w: %s",
			err,
			strings.TrimSpace(string(output)),
		)
		return "", nil, joinCleanupError(
			buildErr,
			"clean failed temporary indexer build",
			cleanup(),
		)
	}
	return path, cleanup, nil
}

func writeExecutionTimings(
	writer io.Writer,
	timings []fpfrefresh.StageTiming,
) error {
	if _, err := fmt.Fprintln(writer, "execution_timings_noncanonical:"); err != nil {
		return err
	}
	for _, timing := range timings {
		if _, err := fmt.Fprintf(
			writer,
			"  %s: %s\n",
			timing.Stage().String(),
			timing.Duration().Round(time.Millisecond),
		); err != nil {
			return err
		}
	}
	return nil
}

func writeNextCommand(
	writer io.Writer,
	root string,
	state fpfrefresh.CheckState,
	candidateSHA string,
) error {
	quotedRoot := fmt.Sprintf("%q", root)
	switch state {
	case fpfrefresh.StateNoChange:
		_, err := fmt.Fprintln(
			writer,
			"next: none; exact candidate is already current",
		)
		return err
	case fpfrefresh.StateApplyReady:
		_, err := fmt.Fprintf(
			writer,
			"next: go run ./cmd/fpf-refresh apply --repo %s --candidate-ref %s --no-fetch\n",
			quotedRoot,
			candidateSHA,
		)
		return err
	case fpfrefresh.StateReviewReady:
		_, err := fmt.Fprintf(
			writer,
			"next: go run ./cmd/fpf-refresh apply --repo %s --candidate-ref %s --no-fetch; semantic deltas remain visible in the report but do not block the source-current baseline\n",
			quotedRoot,
			candidateSHA,
		)
		return err
	case fpfrefresh.StateCandidateRejected:
		_, err := fmt.Fprintln(
			writer,
			"next: resolve the exact reported diagnostic, then rerun check; no apply is available",
		)
		return err
	}
	return nil
}
