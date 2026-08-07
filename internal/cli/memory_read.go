package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
	"github.com/spf13/cobra"
)

var (
	memoryResolveInputFile      string
	memoryNeighborhoodInputFile string
	memoryRecallInputFile       string
)

type boundProjectMemoryRead struct {
	ledger  *projectledger.Handle
	runtime projectMemoryReadRuntime
}

type projectMemoryReadSession interface {
	Execute(
		context.Context,
		typedmemorywire.Request,
	) ([]byte, error)
	Close() error
}

type projectMemoryReadSessionOpener func(
	context.Context,
) (projectMemoryReadSession, error)

var openProjectMemoryReadSession projectMemoryReadSessionOpener = func(
	ctx context.Context,
) (projectMemoryReadSession, error) {
	return openBoundProjectMemoryReadRuntime(ctx)
}

var memoryResolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Resolve an EntityOfConcern in the exact project snapshot",
	Args:  cobra.NoArgs,
	RunE:  runMemoryResolve,
}

var memoryNeighborhoodCmd = &cobra.Command{
	Use:   "neighborhood",
	Short: "Read an exact EntityOfConcern project-memory neighborhood",
	Args:  cobra.NoArgs,
	RunE:  runMemoryNeighborhood,
}

var memoryRecallCmd = &cobra.Command{
	Use:   "recall",
	Short: "Recall candidates inside an exact EntityOfConcern scope",
	Args:  cobra.NoArgs,
	RunE:  runMemoryRecall,
}

func init() {
	memoryResolveCmd.Flags().StringVar(
		&memoryResolveInputFile,
		"input-file",
		"",
		"Strict haft.memory.v1 resolve JSON file, or - for stdin",
	)
	memoryNeighborhoodCmd.Flags().StringVar(
		&memoryNeighborhoodInputFile,
		"input-file",
		"",
		"Strict haft.memory.v1 neighborhood JSON file, or - for stdin",
	)
	memoryRecallCmd.Flags().StringVar(
		&memoryRecallInputFile,
		"input-file",
		"",
		"Strict haft.memory.v1 recall JSON file, or - for stdin",
	)
}

func runMemoryResolve(cmd *cobra.Command, _ []string) error {
	return runProjectMemoryRead(
		cmd,
		memoryResolveInputFile,
		typedmemorywire.ActionResolve,
	)
}

func runMemoryNeighborhood(cmd *cobra.Command, _ []string) error {
	return runProjectMemoryRead(
		cmd,
		memoryNeighborhoodInputFile,
		typedmemorywire.ActionNeighborhood,
	)
}

func runMemoryRecall(cmd *cobra.Command, _ []string) error {
	return runProjectMemoryRead(
		cmd,
		memoryRecallInputFile,
		typedmemorywire.ActionRecall,
	)
}

func runProjectMemoryRead(
	cmd *cobra.Command,
	inputFile string,
	expectedAction string,
) (runErr error) {
	if inputFile == "" {
		return fmt.Errorf(
			"--input-file is required; use --input-file - for stdin",
		)
	}
	payload, err := readMemoryValidationInput(cmd, inputFile)
	if err != nil {
		return err
	}
	request, err := typedmemorywire.DecodeRequest(payload)
	if err != nil {
		return err
	}
	if request.Action() != expectedAction {
		return fmt.Errorf(
			"memory %s requires action %q, got %q",
			expectedAction,
			expectedAction,
			request.Action(),
		)
	}
	session, err := openProjectMemoryReadSession(cmd.Context())
	if err != nil {
		return err
	}
	defer func() {
		closeErr := session.Close()
		runErr = errors.Join(runErr, closeErr)
	}()
	result, err := session.Execute(cmd.Context(), request)
	if err != nil {
		return err
	}
	written, err := cmd.OutOrStdout().Write(result)
	if err == nil && written != len(result) {
		err = io.ErrShortWrite
	}
	return err
}

func openBoundProjectMemoryReadRuntime(
	ctx context.Context,
) (*boundProjectMemoryRead, error) {
	binding, err := resolveProjectMemoryAdmissionRoot()
	if err != nil {
		return nil, projectBindingError(binding, err)
	}
	ledger, err := projectledger.OpenExisting(
		ctx,
		binding.ProjectRoot,
		projectledger.ReadOnly,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open checked Haft project ledger read-only: %w; %s",
			err,
			formatProjectBindingDiagnostic(binding),
		)
	}
	closeWithError := func(
		openErr error,
	) (*boundProjectMemoryRead, error) {
		closeErr := ledger.Close()
		return nil, errors.Join(openErr, closeErr)
	}
	ledgerProjectID := ledger.ProjectID()
	if binding.ExpectedProjectID != "" &&
		ledgerProjectID.String() != binding.ExpectedProjectID {
		openErr := fmt.Errorf(
			"open checked Haft project ledger read-only: %w: expected %q, bound project is %q",
			errExpectedProjectIDMiss,
			binding.ExpectedProjectID,
			ledgerProjectID.String(),
		)
		return closeWithError(openErr)
	}
	runtime, err := newProjectMemoryReadRuntime(
		ctx,
		ledgerProjectID,
		ledger.Database(),
	)
	if err != nil {
		return closeWithError(err)
	}
	if err := ledger.Revalidate(ctx); err != nil {
		openErr := fmt.Errorf(
			"revalidate checked Haft project ledger after read-runtime construction: %w",
			err,
		)
		return closeWithError(openErr)
	}
	return &boundProjectMemoryRead{
		ledger:  ledger,
		runtime: runtime,
	}, nil
}

func (session *boundProjectMemoryRead) Execute(
	ctx context.Context,
	request typedmemorywire.Request,
) ([]byte, error) {
	if session == nil || session.ledger == nil {
		return nil, fmt.Errorf("bound project-memory read is closed")
	}
	if err := session.ledger.Revalidate(ctx); err != nil {
		return nil, fmt.Errorf(
			"revalidate checked Haft project ledger before read: %w",
			err,
		)
	}
	result, err := session.runtime.ExecuteReadOnlyRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	if err := session.ledger.Revalidate(ctx); err != nil {
		return nil, fmt.Errorf(
			"revalidate checked Haft project ledger after read: discard untrusted result: %w",
			err,
		)
	}
	return result, nil
}

func (session *boundProjectMemoryRead) Close() error {
	if session == nil || session.ledger == nil {
		return nil
	}
	ledger := session.ledger
	session.ledger = nil
	return ledger.Close()
}
