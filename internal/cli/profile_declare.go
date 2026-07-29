package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/profileonboarding"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

const profileDeclarationRecordKind = "haft_project_profile_declaration"

var profileDeclareJSON bool
var profileDeclareInputFile string

type profileLedgerRevalidation = func(context.Context) error

var profileDeclareCmd = &cobra.Command{
	Use:   "declare",
	Short: "Declare the reviewed project profile",
	Long: `Declare the readable project-profile review candidate.

By default this reads .haft/profile-declaration-review.json produced by
"haft profile propose". The command derives all durable identities itself.
The default explicit_h_onboard policy treats this explicit invocation as the
human gate. The reserved strict_cli_speech_act mode fails closed until a native
v3 strict profile-authority source is available.`,
	Args: cobra.NoArgs,
	RunE: runProfileDeclare,
}

func init() {
	profileDeclareCmd.Flags().BoolVar(
		&profileDeclareJSON,
		"json",
		false,
		"print the declaration result as structured JSON",
	)
	profileDeclareCmd.Flags().StringVar(
		&profileDeclareInputFile,
		"input-file",
		"",
		"read an explicit review carrier instead of the default proposal",
	)
	profileCmd.AddCommand(profileDeclareCmd)
}

type profileDeclarationRuntime interface {
	declare(
		context.Context,
		*sql.DB,
		string,
		profileonboarding.ProfileOnboardingWorkInput,
		profileonboarding.ProfileDeclarationPolicy,
		profileLedgerRevalidation,
	) (profileOnboardOutcome, error)
}

type canonicalProfileDeclarationRuntime struct{}

func (canonicalProfileDeclarationRuntime) declare(
	ctx context.Context,
	database *sql.DB,
	projectRoot string,
	input profileonboarding.ProfileOnboardingWorkInput,
	policy profileonboarding.ProfileDeclarationPolicy,
	revalidate profileLedgerRevalidation,
) (profileOnboardOutcome, error) {
	result, err := profileonboarding.RunProfileDeclaration(
		ctx,
		database,
		projectRoot,
		input,
		policy,
		func(revalidationContext context.Context) error {
			return revalidate(revalidationContext)
		},
	)
	if err != nil {
		return profileOnboardOutcome{}, err
	}
	return normalizeProfileOnboardResult(result)
}

func runProfileDeclare(cmd *cobra.Command, _ []string) error {
	return runProfileDeclarationCommand(
		cmd,
		profileDeclareInputFile,
		profileDeclareJSON,
	)
}

func runProfileDeclarationCommand(
	cmd *cobra.Command,
	requestedInput string,
	asJSON bool,
) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}
	response, runErr := executeReviewedProfileDeclaration(
		commandContext(cmd),
		projectRoot,
		requestedInput,
	)
	if response.Kind != "" {
		writeErr := writeProfileOnboardResponse(
			cmd.OutOrStdout(),
			response,
			asJSON,
		)
		runErr = errors.Join(runErr, writeErr)
	}
	return runErr
}

// executeReviewedProfileDeclaration is the typed effect boundary shared by
// the low-level diagnostic command and the task-level onboarding adapter. It
// returns the exact closed result even when the canonical admission committed
// but a later projection or attachment check needs attention.
func executeReviewedProfileDeclaration(
	ctx context.Context,
	projectRoot string,
	requestedInput string,
) (profileOnboardResponse, error) {
	inputPath := resolveProfileDeclarationInputPath(
		projectRoot,
		requestedInput,
	)
	content, err := readProfileDeclarationInput(inputPath)
	if err != nil {
		return profileOnboardResponse{}, err
	}
	suggestion, err := profiledetector.Inspect(projectRoot)
	if err != nil {
		return profileOnboardResponse{}, fmt.Errorf(
			"inspect current project-profile evidence: %w",
			err,
		)
	}
	input, err := profileonboarding.DecodeProfileOnboardingWorkInput(
		content,
		suggestion,
	)
	if err != nil {
		return profileOnboardResponse{}, fmt.Errorf(
			"review candidate no longer matches current repository evidence: %w; run `haft profile propose` again after deliberately removing or archiving the stale candidate",
			err,
		)
	}
	policy, err := loadProfileDeclarationPolicy(projectRoot)
	if err != nil {
		return profileOnboardResponse{}, err
	}
	response, runErr := executeProfileDeclaration(
		ctx,
		projectRoot,
		input,
		policy,
		canonicalProfileDeclarationRuntime{},
	)
	if response.Kind != "" {
		response.ReviewInput = profileDeclarationDisplayPath(
			projectRoot,
			inputPath,
		)
		response.AuthorityMode = policy.Mode()
	}
	if runErr != nil {
		return response, runErr
	}
	return response, profileOnboardOutcomeError(response)
}

func executeProfileDeclaration(
	ctx context.Context,
	projectRoot string,
	input profileonboarding.ProfileOnboardingWorkInput,
	policy profileonboarding.ProfileDeclarationPolicy,
	runtime profileDeclarationRuntime,
) (response profileOnboardResponse, runErr error) {
	if ctx == nil {
		return profileOnboardResponse{}, fmt.Errorf(
			"profile declaration requires a context",
		)
	}
	if runtime == nil {
		return profileOnboardResponse{}, fmt.Errorf(
			"profile declaration runtime is unavailable",
		)
	}
	handle, err := projectledger.OpenExisting(
		ctx,
		projectRoot,
		projectledger.ReadWrite,
	)
	if err != nil {
		return profileOnboardResponse{}, fmt.Errorf(
			"open checked project ledger: %w",
			err,
		)
	}
	defer func() {
		runErr = errors.Join(runErr, handle.Close())
	}()
	canonicalRoot := handle.ProjectRoot().String()
	outcome, err := runtime.declare(
		ctx,
		handle.Database(),
		canonicalRoot,
		input,
		policy,
		func(revalidationContext context.Context) error {
			return handle.Revalidate(revalidationContext)
		},
	)
	if err != nil {
		return profileOnboardResponse{}, fmt.Errorf(
			"declare canonical project profile: %w",
			err,
		)
	}
	if err := validateProfileOnboardOutcome(outcome); err != nil {
		return profileOnboardResponse{}, fmt.Errorf(
			"validate canonical profile outcome: %w",
			err,
		)
	}
	response = profileOnboardResponse{
		Kind:          profileDeclarationRecordKind,
		State:         outcome.State,
		ProjectRoot:   canonicalRoot,
		ProjectID:     handle.ProjectID().String(),
		AuthorityMode: policy.Mode(),
		Admission:     outcome.Admission,
		Revision:      outcome.Revision,
		Projection:    outcome.Projection,
		Rejections:    outcome.Rejections,
		Failure:       outcome.Failure,
	}
	return response, nil
}

func loadProfileDeclarationPolicy(
	projectRoot string,
) (profileonboarding.ProfileDeclarationPolicy, error) {
	configPath := project.ProjectConfigPath(filepath.Join(projectRoot, ".haft"))
	content, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		content = []byte(project.ExampleProjectConfigYAML())
		config := project.DefaultProjectConfig()
		mode := config.EffectiveProfileDeclarationMode()
		return profileonboarding.NewProfileDeclarationPolicy(
			string(mode),
			"haft:default-project-config/v1",
			content,
		)
	}
	if err != nil {
		return profileonboarding.ProfileDeclarationPolicy{}, fmt.Errorf(
			"read project profile authority config: %w",
			err,
		)
	}
	config, err := project.ParseProjectConfig(content)
	if err != nil {
		return profileonboarding.ProfileDeclarationPolicy{}, fmt.Errorf(
			"parse project profile authority config: %w",
			err,
		)
	}
	mode := config.EffectiveProfileDeclarationMode()
	return profileonboarding.NewProfileDeclarationPolicy(
		string(mode),
		profileDeclarationConfigRef(),
		content,
	)
}

func resolveProfileDeclarationInputPath(
	projectRoot string,
	requested string,
) string {
	if requested == "" {
		return profileDeclarationReviewPath(projectRoot)
	}
	if filepath.IsAbs(requested) {
		return filepath.Clean(requested)
	}
	return filepath.Clean(filepath.Join(projectRoot, requested))
}

func readProfileDeclarationInput(path string) ([]byte, error) {
	content, present, err := readOptionalRegularProfileReview(path)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, fmt.Errorf(
			"project-profile review candidate %s is absent; run `haft profile propose` first",
			path,
		)
	}
	return content, nil
}

func profileDeclarationDisplayPath(projectRoot string, inputPath string) string {
	relative, err := filepath.Rel(projectRoot, inputPath)
	outside := relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
	if err == nil && !outside && !filepath.IsAbs(relative) {
		return filepath.ToSlash(relative)
	}
	return inputPath
}

func profileDeclarationConfigRef() string {
	return filepath.ToSlash(filepath.Join(".haft", "config.yaml"))
}
