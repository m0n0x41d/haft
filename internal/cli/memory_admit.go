package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
	"github.com/spf13/cobra"
)

var memoryAdmitInputFile string

type boundProjectMemoryAdmission struct {
	ledger  *projectledger.Handle
	runtime projectMemoryRuntime
}

type projectMemoryAdmissionOperation func(
	context.Context,
	typedmemorywire.AdmitRequest,
) ([]byte, error)

type projectMemoryIdentityRevalidator func(context.Context) error

type projectBindingIdentityRequirement func(ProjectBinding, string) error

type projectMemoryAdmissionSession interface {
	Admit(
		context.Context,
		typedmemorywire.AdmitRequest,
	) ([]byte, error)
	Close() error
}

type projectMemoryAdmissionSessionOpener func(
	context.Context,
) (projectMemoryAdmissionSession, error)

var openProjectMemoryAdmissionSession projectMemoryAdmissionSessionOpener = func(
	ctx context.Context,
) (projectMemoryAdmissionSession, error) {
	return openBoundProjectMemoryRuntime(ctx)
}

var memoryAdmitCmd = &cobra.Command{
	Use:   "admit",
	Short: "Admit an exact non-binding typed-memory change set",
	Args:  cobra.NoArgs,
	RunE:  runMemoryAdmit,
}

func init() {
	memoryAdmitCmd.Flags().StringVar(
		&memoryAdmitInputFile,
		"input-file",
		"",
		"Strict haft.memory.v2 admission JSON file, or - for stdin",
	)
}

func runMemoryAdmit(cmd *cobra.Command, _ []string) (runErr error) {
	if memoryAdmitInputFile == "" {
		return fmt.Errorf("--input-file is required; use --input-file - for stdin")
	}
	payload, err := readMemoryValidationInput(cmd, memoryAdmitInputFile)
	if err != nil {
		return err
	}
	request, err := typedmemorywire.DecodeAdmitRequest(payload)
	if err != nil {
		return err
	}
	admission, err := openProjectMemoryAdmissionSession(cmd.Context())
	if err != nil {
		return err
	}
	defer func() {
		closeErr := admission.Close()
		runErr = errors.Join(runErr, closeErr)
	}()
	result, admitErr := admission.Admit(cmd.Context(), request)
	if len(result) == 0 {
		return admitErr
	}
	written, writeErr := cmd.OutOrStdout().Write(result)
	if writeErr == nil && written != len(result) {
		writeErr = io.ErrShortWrite
	}
	return errors.Join(admitErr, writeErr)
}

func openBoundProjectMemoryRuntime(
	ctx context.Context,
) (*boundProjectMemoryAdmission, error) {
	binding, err := resolveProjectMemoryAdmissionRoot()
	if err != nil {
		return nil, projectBindingError(
			binding,
			err,
		)
	}
	return openBoundProjectMemoryRuntimeFromDiscovery(ctx, binding)
}

// openBoundProjectMemoryRuntimeFromBinding consumes a project binding that
// the outer shell has already resolved. It never consults cwd or environment
// variables. The anchored ledger remains the authority for the actual project
// identity and store topology.
func openBoundProjectMemoryRuntimeFromBinding(
	ctx context.Context,
	binding ProjectBinding,
) (*boundProjectMemoryAdmission, error) {
	return openBoundProjectMemoryRuntimeWithIdentityRequirement(
		ctx,
		binding,
		requireResolvedProjectBindingIdentity,
	)
}

func openBoundProjectMemoryRuntimeFromDiscovery(
	ctx context.Context,
	binding ProjectBinding,
) (*boundProjectMemoryAdmission, error) {
	return openBoundProjectMemoryRuntimeWithIdentityRequirement(
		ctx,
		binding,
		requireExpectedProjectBindingIdentity,
	)
}

func openBoundProjectMemoryRuntimeWithIdentityRequirement(
	ctx context.Context,
	binding ProjectBinding,
	requireIdentity projectBindingIdentityRequirement,
) (*boundProjectMemoryAdmission, error) {
	if requireIdentity == nil {
		return nil, fmt.Errorf(
			"open checked haft project ledger: identity requirement is missing",
		)
	}
	ledger, err := projectledger.OpenExisting(
		ctx,
		binding.ProjectRoot,
		projectledger.ReadWrite,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open checked haft project ledger: %w; %s",
			err,
			formatProjectBindingDiagnostic(binding),
		)
	}
	closeWithError := func(openErr error) (*boundProjectMemoryAdmission, error) {
		closeErr := ledger.Close()
		return nil, errors.Join(openErr, closeErr)
	}
	ledgerProjectID := ledger.ProjectID()
	if err := requireIdentity(
		binding,
		ledgerProjectID.String(),
	); err != nil {
		openErr := fmt.Errorf("open checked haft project ledger: %w", err)
		return closeWithError(openErr)
	}
	binding.ProjectID = ledgerProjectID.String()
	database := ledger.Database()
	runtime, err := newProjectMemoryRuntime(
		ctx,
		ledgerProjectID,
		database,
	)
	if err != nil {
		return closeWithError(err)
	}
	if err := ledger.Revalidate(ctx); err != nil {
		openErr := fmt.Errorf(
			"revalidate checked haft project ledger after project-memory construction: %w",
			err,
		)
		return closeWithError(openErr)
	}
	return &boundProjectMemoryAdmission{
		ledger:  ledger,
		runtime: runtime,
	}, nil
}

func requireResolvedProjectBindingIdentity(
	binding ProjectBinding,
	ledgerProjectID string,
) error {
	resolvedProjectID := strings.TrimSpace(binding.ProjectID)
	if resolvedProjectID == "" {
		return fmt.Errorf(
			"resolved project binding has no project_id; bound project is %q",
			ledgerProjectID,
		)
	}
	if ledgerProjectID != resolvedProjectID {
		return fmt.Errorf(
			"resolved project binding identity mismatch: resolved %q, bound project is %q",
			resolvedProjectID,
			ledgerProjectID,
		)
	}
	expectedProjectID := strings.TrimSpace(binding.ExpectedProjectID)
	if expectedProjectID == "" {
		return nil
	}
	if ledgerProjectID == expectedProjectID {
		return nil
	}
	return fmt.Errorf(
		"%w: expected %q, bound project is %q",
		errExpectedProjectIDMiss,
		expectedProjectID,
		ledgerProjectID,
	)
}

func requireExpectedProjectBindingIdentity(
	binding ProjectBinding,
	ledgerProjectID string,
) error {
	expectedProjectID := strings.TrimSpace(binding.ExpectedProjectID)
	if expectedProjectID == "" || expectedProjectID == ledgerProjectID {
		return nil
	}
	return fmt.Errorf(
		"%w: expected %q, bound project is %q",
		errExpectedProjectIDMiss,
		expectedProjectID,
		ledgerProjectID,
	)
}

// resolveProjectMemoryAdmissionRoot deliberately stops before Config.DBPath:
// that legacy resolver may create the project-store directory or rename a
// legacy database. The dormant admission opener accepts only an already
// existing, identity-bound project ledger, so anchored ledger discovery owns
// the project ID and every store-path decision.
func resolveProjectMemoryAdmissionRoot() (ProjectBinding, error) {
	input, err := projectRootInputFromEnv()
	if err != nil {
		return ProjectBinding{}, err
	}
	expectedProjectID := strings.TrimSpace(os.Getenv(envExpectedProjectID))
	binding := ProjectBinding{
		RootSource:        input.Source,
		SearchStart:       input.Path,
		ExpectedProjectID: expectedProjectID,
		DBState:           "unchecked_existing_only",
	}
	root, err := findProjectRootFrom(input.Path)
	if err != nil {
		return binding, fmt.Errorf(
			"resolve project root from %s=%q: %w",
			input.Source,
			input.Path,
			err,
		)
	}
	binding.ProjectRoot = root
	binding.HaftDir = filepath.Join(root, ".haft")
	return binding, nil
}

func (admission *boundProjectMemoryAdmission) Admit(
	ctx context.Context,
	request typedmemorywire.AdmitRequest,
) ([]byte, error) {
	if admission == nil || admission.ledger == nil {
		return nil, fmt.Errorf("bound project-memory admission is closed")
	}
	return admitBoundProjectMemory(
		ctx,
		admission.ledger.Revalidate,
		admission.runtime.Admit,
		request,
	)
}

func (admission *boundProjectMemoryAdmission) Close() error {
	if admission == nil || admission.ledger == nil {
		return nil
	}
	ledger := admission.ledger
	admission.ledger = nil
	return ledger.Close()
}

func admitBoundProjectMemory(
	ctx context.Context,
	revalidate projectMemoryIdentityRevalidator,
	admit projectMemoryAdmissionOperation,
	request typedmemorywire.AdmitRequest,
) ([]byte, error) {
	if revalidate == nil || admit == nil {
		return nil, fmt.Errorf("bound project-memory admission dependencies are incomplete")
	}
	if err := revalidate(ctx); err != nil {
		return nil, fmt.Errorf(
			"revalidate checked haft project ledger before admission: %w",
			err,
		)
	}
	result, admitErr := admit(ctx, request)
	postErr := revalidate(ctx)
	if postErr == nil {
		return result, admitErr
	}
	detail := "the admission outcome is unknown and must not be reported as no-write"
	if len(result) > 0 {
		detail = "the admission returned a result that may already be durable and must not be reported as no-write"
	}
	identityErr := fmt.Errorf(
		"revalidate checked haft project ledger after admission: %s: %w",
		detail,
		postErr,
	)
	return result, errors.Join(admitErr, identityErr)
}
