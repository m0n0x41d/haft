package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
	"github.com/spf13/cobra"
)

var memoryValidateInputFile string

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Validate, admit, and read typed project memory",
}

var memoryValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a typed-memory change set without persisting it",
	Long: `Validate one strict haft.memory.v2 request without persistence or authority.

The input is read byte-for-byte from --input-file. Use --input-file - for
stdin. Project-current and exact-project selectors require an existing selected
ProjectTypeEnvHead and never create, migrate, repair, or select it. Without that
basis they return structured project_basis_unavailable. The
bundled_candidate_open_world selector can establish only structural lowering
against the embedded source-derived TypeEnv; it returns Underdetermined because
it is not a project memory snapshot.`,
	Args: cobra.NoArgs,
	RunE: runMemoryValidate,
}

type readOnlyMemoryValidation struct {
	service typedmemoryvalidation.Service
}

type prePersistenceMemoryBasisResolver struct {
	bundled *typedmemoryvalidation.BundledCandidateOpenWorldBasis
}

func init() {
	memoryValidateCmd.Flags().StringVar(
		&memoryValidateInputFile,
		"input-file",
		"",
		"Strict haft.memory.v2 JSON file, or - for stdin",
	)
	memoryTypeEnvCmd.AddCommand(
		memoryTypeEnvPrepareCmd,
		memoryTypeEnvSelectCmd,
	)
	memoryCmd.AddCommand(
		memoryValidateCmd,
		memoryAdmitCmd,
		memoryResolveCmd,
		memoryNeighborhoodCmd,
		memoryRecallCmd,
		memoryTypeEnvCmd,
	)
	rootCmd.AddCommand(memoryCmd)
}

func runMemoryValidate(cmd *cobra.Command, _ []string) error {
	if memoryValidateInputFile == "" {
		return fmt.Errorf("--input-file is required; use --input-file - for stdin")
	}

	payload, err := readMemoryValidationInput(cmd, memoryValidateInputFile)
	if err != nil {
		return err
	}
	request, err := typedmemorywire.DecodeValidateRequest(payload)
	if err != nil {
		return err
	}
	result, err := validateMemoryRequest(cmd.Context(), request)
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(result)
	return err
}

func validateMemoryRequest(
	ctx context.Context,
	request typedmemorywire.ValidateRequest,
) (result []byte, resultErr error) {
	switch request.Basis().(type) {
	case typedmemorywire.BundledCandidateOpenWorldSelector:
		runtime, err := newReadOnlyMemoryValidation(ctx)
		if err != nil {
			return nil, err
		}
		return runtime.ValidateDecoded(request)
	case typedmemorywire.ProjectCurrentSelector,
		typedmemorywire.ExactProjectSelector:
		session, err := openProjectMemoryReadSession(ctx)
		if err != nil {
			return nil, err
		}
		defer func() {
			resultErr = errors.Join(resultErr, session.Close())
		}()
		return session.Execute(ctx, request)
	default:
		return nil, fmt.Errorf(
			"typed-memory basis %q is unsupported",
			request.Basis().Kind(),
		)
	}
}

func readMemoryValidationInput(
	cmd *cobra.Command,
	inputFile string,
) ([]byte, error) {
	if inputFile == "-" {
		return readBoundedMemoryValidationInput(cmd.InOrStdin(), "stdin")
	}

	file, err := os.Open(inputFile)
	if err != nil {
		return nil, fmt.Errorf("open memory validation input %q: %w", inputFile, err)
	}
	defer file.Close()
	return readBoundedMemoryValidationInput(file, inputFile)
}

func readBoundedMemoryValidationInput(
	reader io.Reader,
	source string,
) ([]byte, error) {
	limit := int64(typedmemorywire.MaximumRequestBytes + 1)
	payload, err := io.ReadAll(io.LimitReader(reader, limit))
	if err != nil {
		return nil, fmt.Errorf("read memory validation input %q: %w", source, err)
	}
	return payload, nil
}

func newReadOnlyMemoryValidation(
	ctx context.Context,
) (readOnlyMemoryValidation, error) {
	runtime, err := loadEmbeddedMemoryRuntime(ctx)
	if err != nil {
		return readOnlyMemoryValidation{}, fmt.Errorf(
			"load bundled typed-memory candidate: %w",
			err,
		)
	}
	bundled, err := typedmemoryvalidation.NewBundledCandidateOpenWorldBasis(
		runtime.Environment(),
		runtime.CodecRegistry(),
	)
	if err != nil {
		return readOnlyMemoryValidation{}, fmt.Errorf(
			"construct bundled typed-memory candidate: %w",
			err,
		)
	}
	resolver := prePersistenceMemoryBasisResolver{bundled: bundled}
	service, err := typedmemoryvalidation.NewService(resolver)
	if err != nil {
		return readOnlyMemoryValidation{}, fmt.Errorf(
			"construct typed-memory validation service: %w",
			err,
		)
	}
	return readOnlyMemoryValidation{service: service}, nil
}

func (resolver prePersistenceMemoryBasisResolver) Resolve(
	selector typedmemorywire.BasisSelector,
) typedmemoryvalidation.BasisResolution {
	switch selector.(type) {
	case typedmemorywire.BundledCandidateOpenWorldSelector:
		return resolver.bundled
	case typedmemorywire.ProjectCurrentSelector,
		typedmemorywire.ExactProjectSelector:
		return typedmemoryvalidation.NewProjectBasisUnavailable()
	default:
		return typedmemoryvalidation.NewProjectBasisUnavailable()
	}
}

func (runtime readOnlyMemoryValidation) Validate(payload []byte) ([]byte, error) {
	request, err := typedmemorywire.DecodeValidateRequest(payload)
	if err != nil {
		return nil, err
	}
	return runtime.ValidateDecoded(request)
}

func (runtime readOnlyMemoryValidation) ValidateDecoded(
	request typedmemorywire.ValidateRequest,
) ([]byte, error) {
	response := runtime.service.Validate(request)
	encoded, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("encode memory validation response: %w", err)
	}
	return append(encoded, '\n'), nil
}

func (runtime readOnlyMemoryValidation) MCPHandler() fpf.MemoryToolHandler {
	return func(ctx context.Context, arguments json.RawMessage) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		result, err := runtime.Validate(arguments)
		if err != nil {
			return "", err
		}
		return string(result), nil
	}
}

func (runtime readOnlyMemoryValidation) PreProjectMCPHandler() fpf.MemoryToolHandler {
	return func(ctx context.Context, arguments json.RawMessage) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		request, err := typedmemorywire.DecodeRequest(arguments)
		if err != nil {
			return "", err
		}
		switch exact := request.(type) {
		case typedmemorywire.ValidateRequest:
			result, validateErr := runtime.ValidateDecoded(exact)
			return string(result), validateErr
		case typedmemorywire.AdmitRequest:
			result, recoveryErr := projectMemoryRecoveryResponse(
				exact.ContractVersion(),
				exact.Action(),
				"",
				"project_basis_unavailable",
				"Haft is not initialized with structured project memory; "+
					"no memory write was performed.",
			)
			return string(result), recoveryErr
		default:
			return "", fmt.Errorf(
				"haft_memory action %q is unavailable on the validate/admit boundary",
				request.Action(),
			)
		}
	}
}

func configureMemoryValidation(
	ctx context.Context,
	server *fpf.Server,
) error {
	if server == nil {
		return fmt.Errorf("configure typed-memory validation: server is required")
	}
	runtime, err := newReadOnlyMemoryValidation(ctx)
	if err != nil {
		return err
	}
	server.SetMemoryFullHandler(runtime.PreProjectMCPHandler())
	server.SetMemoryReadHandler(newProjectMemoryUnavailableReadMCPHandler())
	return nil
}
