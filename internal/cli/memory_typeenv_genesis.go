package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvpreparation"
	preparationsqlite "github.com/m0n0x41d/haft/internal/projecttypeenvpreparation/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvreviewcarrier"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	selectionsqlite "github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/spf13/cobra"
)

const (
	projectTypeEnvGenesisReviewSchema              = "haft.project-typeenv.genesis-review/v1"
	projectTypeEnvGenesisReviewFileName            = projecttypeenvreviewcarrier.FileName
	projectTypeEnvGenesisReviewCarrierAuthorityRef = "carrier:.haft/" + projecttypeenvreviewcarrier.FileName
	projectTypeEnvGenesisReviewWindow              = 24 * time.Hour
	maximumProjectTypeEnvGenesisReviewBytes        = projecttypeenvreviewcarrier.MaximumBytes
)

var memoryTypeEnvCmd = &cobra.Command{
	Use:    "typeenv",
	Short:  "Prepare and select the project's exact typed-memory ontology",
	Hidden: true,
}

var memoryTypeEnvPrepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "Prepare a non-binding project TypeEnv review",
	Args:  cobra.NoArgs,
	RunE:  runMemoryTypeEnvPrepare,
}

var memoryTypeEnvSelectCmd = &cobra.Command{
	Use:   "select",
	Short: "Select the exact reviewed project TypeEnv after manual h-decide",
	Args:  cobra.NoArgs,
	RunE:  runMemoryTypeEnvSelect,
}

var memoryTypeEnvPrepareReplaceReview bool

func init() {
	memoryTypeEnvPrepareCmd.Flags().BoolVar(
		&memoryTypeEnvPrepareReplaceReview,
		"replace-review",
		false,
		"Replace a different existing non-binding TypeEnv review carrier",
	)
}

type projectTypeEnvGenesisReviewCarrier struct {
	Schema                              string                                    `json:"schema"`
	ProjectID                           string                                    `json:"project_id"`
	PreparationResult                   string                                    `json:"preparation_result"`
	Review                              projectTypeEnvGenesisHumanReview          `json:"review"`
	Candidate                           projectTypeEnvGenesisCandidateResponse    `json:"candidate"`
	Interpretation                      projectTypeEnvGenesisReviewInterpretation `json:"interpretation"`
	StageRef                            string                                    `json:"stage_ref"`
	RequestRef                          string                                    `json:"request_ref"`
	RequestDigest                       string                                    `json:"request_digest"`
	RequestCanonicalBase64              string                                    `json:"request_canonical_base64"`
	AuthorizationContentDigest          string                                    `json:"authorization_content_digest"`
	AuthorizationContentCanonicalBase64 string                                    `json:"authorization_content_canonical_base64"`
	PreparedAt                          string                                    `json:"prepared_at"`
	ExpiresAt                           string                                    `json:"expires_at"`
}

type projectTypeEnvGenesisReviewResponse struct {
	ContractVersion               string                                      `json:"contract_version"`
	Action                        string                                      `json:"action"`
	Result                        string                                      `json:"result"`
	ProjectID                     string                                      `json:"project_id"`
	Review                        projectTypeEnvGenesisHumanReview            `json:"review"`
	Candidate                     projectTypeEnvGenesisCandidateResponse      `json:"candidate"`
	ReviewCarrier                 projectTypeEnvGenesisReviewCarrierRef       `json:"review_carrier"`
	PostPrepareLedgerRevalidation projectTypeEnvGenesisPostEffectRevalidation `json:"post_prepare_ledger_revalidation"`
	Interpretation                projectTypeEnvGenesisReviewInterpretation   `json:"interpretation"`
}

type projectTypeEnvGenesisHumanReview struct {
	Title           string                               `json:"title"`
	Choice          string                               `json:"choice"`
	WhyNow          string                               `json:"why_now"`
	Changes         []string                             `json:"changes"`
	DoesNotChange   []string                             `json:"does_not_change"`
	Validity        projectTypeEnvGenesisReviewValidity  `json:"validity"`
	Readiness       projectTypeEnvGenesisReviewReadiness `json:"readiness"`
	ReturnCondition string                               `json:"return_condition"`
}

type projectTypeEnvGenesisReviewValidity struct {
	From  string `json:"from"`
	Until string `json:"until"`
}

type projectTypeEnvGenesisReviewReadiness struct {
	Posture string   `json:"posture"`
	Reasons []string `json:"reasons"`
	Repair  string   `json:"repair"`
}

type projectTypeEnvGenesisCandidateResponse struct {
	StageRef                  string `json:"stage_ref"`
	BaseTypeEnvRef            string `json:"base_type_env_ref"`
	ExtensionCount            int    `json:"extension_count"`
	RuntimeEvaluationBasisRef string `json:"runtime_evaluation_basis_ref"`
	CompositeTypeEnvRef       string `json:"composite_type_env_ref"`
	GraphRevision             uint64 `json:"graph_revision"`
	CompatibilityPosture      string `json:"compatibility_posture"`
	RevalidationPosture       string `json:"revalidation_posture"`
	ProfilePosture            string `json:"profile_posture"`
}

type projectTypeEnvGenesisReviewCarrierRef struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type projectTypeEnvGenesisReviewInterpretation struct {
	Establishes      []string `json:"establishes"`
	DoesNotEstablish []string `json:"does_not_establish"`
	NextHumanGate    string   `json:"next_human_gate"`
}

type projectTypeEnvGenesisSelectionResponse struct {
	ContractVersion              string                                       `json:"contract_version"`
	Action                       string                                       `json:"action"`
	ProjectID                    string                                       `json:"project_id"`
	AuthorityIngress             string                                       `json:"authority_ingress"`
	Outcome                      projectTypeEnvGenesisSelectionOutcome        `json:"outcome"`
	PostEffectLedgerRevalidation projectTypeEnvGenesisPostEffectRevalidation  `json:"post_effect_ledger_revalidation"`
	Interpretation               projectTypeEnvGenesisSelectionInterpretation `json:"interpretation"`
}

type projectTypeEnvGenesisSelectionOutcome interface {
	projectTypeEnvGenesisSelectionOutcomeVariant()
}

type projectTypeEnvGenesisFreshlyCommitted struct {
	Kind             string                                    `json:"kind"`
	DeliveryPosture  string                                    `json:"delivery_posture"`
	CommittedClosure projectTypeEnvGenesisCommittedClosureWire `json:"committed_closure"`
}

func (projectTypeEnvGenesisFreshlyCommitted) projectTypeEnvGenesisSelectionOutcomeVariant() {
}

type projectTypeEnvGenesisReplayedExisting struct {
	Kind             string                                    `json:"kind"`
	CommittedClosure projectTypeEnvGenesisCommittedClosureWire `json:"committed_closure"`
}

func (projectTypeEnvGenesisReplayedExisting) projectTypeEnvGenesisSelectionOutcomeVariant() {
}

type projectTypeEnvGenesisNotSelected struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Repair string `json:"repair"`
}

func (projectTypeEnvGenesisNotSelected) projectTypeEnvGenesisSelectionOutcomeVariant() {
}

type projectTypeEnvGenesisReplayConflict struct {
	Kind                   string `json:"kind"`
	IdempotencyKey         string `json:"idempotency_key"`
	ExistingRequestDigest  string `json:"existing_request_digest"`
	PresentedRequestDigest string `json:"presented_request_digest"`
	ExistingContentDigest  string `json:"existing_content_digest"`
	PresentedContentDigest string `json:"presented_content_digest"`
	Repair                 string `json:"repair"`
}

func (projectTypeEnvGenesisReplayConflict) projectTypeEnvGenesisSelectionOutcomeVariant() {
}

type projectTypeEnvGenesisCommitOutcomeUnknown struct {
	Kind          string `json:"kind"`
	RetryKey      string `json:"retry_key"`
	RequestDigest string `json:"request_digest"`
	ContentDigest string `json:"content_digest"`
	Repair        string `json:"repair"`
}

func (projectTypeEnvGenesisCommitOutcomeUnknown) projectTypeEnvGenesisSelectionOutcomeVariant() {
}

type projectTypeEnvGenesisCommittedClosureWire struct {
	ClosureRef             string `json:"closure_ref"`
	ClosureDigest          string `json:"closure_digest"`
	TransactionRef         string `json:"transaction_ref"`
	TransactionDigest      string `json:"transaction_digest"`
	IdempotencyKey         string `json:"idempotency_key"`
	RequestRef             string `json:"request_ref"`
	RequestDigest          string `json:"request_digest"`
	AuthorityBasisKind     string `json:"authority_basis_kind"`
	SelectedCompositeRef   string `json:"selected_composite_ref"`
	HeadRef                string `json:"head_ref"`
	HeadRevision           uint64 `json:"head_revision"`
	ExpectedGraphRevision  uint64 `json:"expected_graph_revision"`
	CommittedGraphRevision uint64 `json:"committed_graph_revision"`
	AuthorityUseRecordRef  string `json:"authority_use_record_ref"`
	WorkRef                string `json:"work_ref"`
	CASWorkRecordRef       string `json:"cas_work_record_ref"`
	GraphEventRef          string `json:"graph_event_ref"`
	GraphCommitRef         string `json:"graph_commit_ref"`
	ReceiptRef             string `json:"receipt_ref"`
	ReceiptDigest          string `json:"receipt_digest"`
	CommittedResultRef     string `json:"committed_result_ref"`
	CommittedResultDigest  string `json:"committed_result_digest"`
}

type projectTypeEnvGenesisPostEffectRevalidation interface {
	projectTypeEnvGenesisPostEffectRevalidationVariant()
}

type projectTypeEnvGenesisLedgerVerifiedAfterEffect struct {
	Kind string `json:"kind"`
}

func (projectTypeEnvGenesisLedgerVerifiedAfterEffect) projectTypeEnvGenesisPostEffectRevalidationVariant() {
}

type projectTypeEnvGenesisLedgerFailedAfterEffect struct {
	Kind   string `json:"kind"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Repair string `json:"repair"`
}

func (projectTypeEnvGenesisLedgerFailedAfterEffect) projectTypeEnvGenesisPostEffectRevalidationVariant() {
}

type projectTypeEnvGenesisSelectionInterpretation struct {
	Establishes      []string `json:"establishes"`
	DoesNotEstablish []string `json:"does_not_establish"`
	DoesNotAuthorize []string `json:"does_not_authorize"`
}

type preparedProjectTypeEnvGenesisReview struct {
	carrier  projectTypeEnvGenesisReviewCarrier
	response projectTypeEnvGenesisReviewResponse
}

type observedProjectTypeEnvGenesisReview struct {
	carrier projectTypeEnvGenesisReviewCarrier
	binding authority.ObservableCarrierBinding
}

func runMemoryTypeEnvPrepare(
	cmd *cobra.Command,
	_ []string,
) (runErr error) {
	ledger, binding, err := openProjectTypeEnvGenesisLedger(
		cmd.Context(),
		projectledger.ReadWrite,
	)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := ledger.Close()
		runErr = errors.Join(runErr, closeErr)
	}()

	runtime, err := loadEmbeddedMemoryRuntime(cmd.Context())
	if err != nil {
		return err
	}
	clock := typedmemorystore.SystemClock{}
	handled, transitionErr := tryRunMemoryTypeEnvTransitionPrepare(
		cmd,
		ledger,
		binding,
		runtime.Artifact(),
		clock,
	)
	if handled || transitionErr != nil {
		return transitionErr
	}
	prepared, err := prepareProjectTypeEnvGenesisReview(
		cmd.Context(),
		ledger,
		runtime.Artifact(),
		clock,
	)
	if err != nil && prepared.response.ContractVersion == "" {
		return err
	}
	if err != nil {
		response := prepared.response
		response.Result = "prepared_but_topology_uncertain"
		response.Review.Readiness = projectTypeEnvGenesisReviewReadiness{
			Posture: "blocked",
			Reasons: []string{
				"project topology could not be revalidated after durable preparation",
			},
			Repair: "restore the canonical project root-to-ledger binding, inspect the retained revision-zero and B/E/X/C/Stage state, then prepare a fresh review",
		}
		response.Interpretation.NextHumanGate = ""
		response.Interpretation.DoesNotEstablish = append(
			response.Interpretation.DoesNotEstablish,
			"the current root-to-ledger attachment after preparation",
		)
		response.PostPrepareLedgerRevalidation =
			genesisLedgerRevalidationResult(err)
		writeErr := writeJSON(cmd.OutOrStdout(), response)
		return errors.Join(err, writeErr)
	}
	install := writeProjectTypeEnvGenesisReview
	if memoryTypeEnvPrepareReplaceReview {
		install = replaceProjectTypeEnvGenesisReview
	}
	digest, err := install(binding.ProjectRoot, prepared.carrier)
	if err != nil {
		return err
	}
	response := prepared.response
	response.ReviewCarrier = projectTypeEnvGenesisReviewCarrierRef{
		Path:   projectTypeEnvGenesisReviewRelativePath(),
		Digest: digest,
	}
	revalidationErr := ledger.Revalidate(cmd.Context())
	response.PostPrepareLedgerRevalidation =
		genesisLedgerRevalidationResult(revalidationErr)
	writeErr := writeJSON(cmd.OutOrStdout(), response)
	if revalidationErr == nil {
		return writeErr
	}
	return errors.Join(
		fmt.Errorf(
			"revalidate project after writing Genesis review: the review carrier was emitted but the current root-to-ledger attachment is unverified: %w",
			revalidationErr,
		),
		writeErr,
	)
}

func runMemoryTypeEnvSelect(
	cmd *cobra.Command,
	_ []string,
) error {
	return runMemoryTypeEnvSelectWithCapturer(
		cmd,
		projecttypeenvselectionauthority.
			ControllingTerminalStrictCLISpeechActCapturer{},
	)
}

func runMemoryTypeEnvSelectWithCapturer(
	cmd *cobra.Command,
	capturer projecttypeenvselectionauthority.StrictCLISpeechActCapturer,
) error {
	response, runErr := executeReviewedMemorySelection(
		commandContext(cmd),
		capturer,
	)
	if response.ContractVersion != "" {
		writeErr := writeJSON(cmd.OutOrStdout(), response)
		runErr = errors.Join(runErr, writeErr)
	}
	return runErr
}

// executeReviewedMemorySelection is the typed effect boundary shared by the
// hidden diagnostic command and the readable onboarding adapter. Rendering is
// deliberately outside this function so task-level callers never depend on or
// round-trip the low-level JSON wire.
func executeReviewedMemorySelection(
	ctx context.Context,
	capturer projecttypeenvselectionauthority.StrictCLISpeechActCapturer,
) (
	response projectTypeEnvGenesisSelectionResponse,
	runErr error,
) {
	ledger, binding, err := openProjectTypeEnvGenesisLedger(
		ctx,
		projectledger.ReadWrite,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	defer func() {
		closeErr := ledger.Close()
		runErr = errors.Join(runErr, closeErr)
	}()
	reviewSchema, err := projectTypeEnvReviewSchema(binding.ProjectRoot)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	if reviewSchema == projectTypeEnvTransitionReviewSchema {
		return executeMemoryTypeEnvTransitionSelection(
			ctx,
			ledger,
			binding,
			capturer,
		)
	}
	if reviewSchema != projectTypeEnvGenesisReviewSchema {
		return projectTypeEnvGenesisSelectionResponse{}, fmt.Errorf(
			"TypeEnv review schema %q is unsupported",
			reviewSchema,
		)
	}

	observedReview, err := observeProjectTypeEnvGenesisReview(
		binding.ProjectRoot,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	carrier := observedReview.carrier
	request, content, stage, err := decodeProjectTypeEnvGenesisReview(
		ctx,
		ledger,
		carrier,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	if err := stage.Verify(); err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, fmt.Errorf(
			"verify reviewed Genesis Stage: %w",
			err,
		)
	}
	target, err := loadExactReviewedMemoryTarget(
		ctx,
		ledger,
		stage,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, fmt.Errorf(
			"construct installed Genesis runtime: %w",
			err,
		)
	}
	if target.Composite().Ref() != request.Target().VerifiedComposite() {
		return projectTypeEnvGenesisSelectionResponse{}, fmt.Errorf(
			"current installed Local-Practice composite differs from reviewed Genesis target",
		)
	}
	service, err := selectionsqlite.NewGenesisService(
		ctx,
		ledger.Database(),
		binding.ProjectRoot,
		target.InstalledRuntime(),
		typedmemorystore.SystemClock{},
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	ingress, err := service.ResolveCurrentCLIIngress(
		ctx,
		request,
		content,
		stage,
		observedReview.binding,
		capturer,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	result, err := service.SelectGenesis(
		ctx,
		selectionsqlite.GenesisSelectionInput{
			Request:   request,
			Content:   content,
			Authority: ingress.Ingress(),
		},
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	response, err = projectTypeEnvGenesisResultResponse(
		ledger.ProjectID().String(),
		result,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	ingressPosture, err := projectTypeEnvGenesisCLIIngressPosture(ingress)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	response.AuthorityIngress = ingressPosture
	revalidationErr := ledger.Revalidate(ctx)
	response.PostEffectLedgerRevalidation =
		genesisLedgerRevalidationResult(revalidationErr)
	if revalidationErr == nil {
		return response, nil
	}
	return response, fmt.Errorf(
		"revalidate project after Genesis selection: the exact effect outcome was emitted but the current root-to-ledger attachment is unverified: %w",
		revalidationErr,
	)
}

func loadExactReviewedMemoryTarget(
	ctx context.Context,
	ledger *projectledger.Handle,
	stage projecttypeenvselection.ProjectTypeEnvStage,
) (localpracticeruntime.Target, error) {
	empty := localpracticeruntime.Target{}
	store, err := projecttypeenvstore.OpenExisting(ctx, ledger.Database())
	if err != nil {
		return empty, fmt.Errorf("open reviewed project TypeEnv artifacts: %w", err)
	}
	base, err := store.GetBaseTypeEnvArtifact(ctx, stage.Base())
	if err != nil {
		return empty, fmt.Errorf("load reviewed Stage base: %w", err)
	}
	sources := typedmemorycandidates.SourcesForExactBaseTypeEnvRef(
		stage.Base().String(),
	)
	if len(sources) == 0 {
		return empty, fmt.Errorf(
			"reviewed Stage base %q has no exact installed Local-Practice source",
			stage.Base().String(),
		)
	}
	matches := make([]localpracticeruntime.Target, 0, len(sources))
	for index, source := range sources {
		target, buildErr := localpracticeruntime.Build(base, source)
		if buildErr != nil {
			return empty, fmt.Errorf(
				"build exact reviewed Local-Practice target candidate %d: %w",
				index,
				buildErr,
			)
		}
		if reviewedTargetMatchesStage(target, stage) {
			matches = append(matches, target)
		}
	}
	if len(matches) != 1 {
		return empty, fmt.Errorf(
			"reviewed B has %d installed Local-Practice targets matching exact E/X/C coordinates; want 1",
			len(matches),
		)
	}
	return matches[0], nil
}

func reviewedTargetMatchesStage(
	target localpracticeruntime.Target,
	stage projecttypeenvselection.ProjectTypeEnvStage,
) bool {
	orderedExtensions := stage.OrderedExtensions()
	return len(orderedExtensions) == 1 &&
		orderedExtensions[0] == target.Extension().Ref() &&
		stage.RuntimeBasis() == target.RuntimeBasis().Ref() &&
		stage.VerifiedComposite() == target.Composite().Ref()
}

func projectTypeEnvGenesisCLIIngressPosture(
	resolution selectionsqlite.CurrentCLIIngressResolution,
) (string, error) {
	switch {
	case resolution.ExplicitHDecide():
		return "explicit_h_decide", nil
	case resolution.StrictCaptured():
		return "strict_speech_act_captured", nil
	case resolution.StrictReplayed():
		return "strict_speech_act_replayed", nil
	default:
		return "", fmt.Errorf("genesis CLI ingress posture is invalid")
	}
}

func genesisLedgerRevalidationResult(
	cause error,
) projectTypeEnvGenesisPostEffectRevalidation {
	if cause == nil {
		return projectTypeEnvGenesisLedgerVerifiedAfterEffect{
			Kind: "verified_after_effect",
		}
	}
	return projectTypeEnvGenesisLedgerFailedAfterEffect{
		Kind:   "failed_after_effect",
		Code:   "project_ledger_revalidation_failed",
		Detail: cause.Error(),
		Repair: "restore or reopen the canonical project binding, then retry the unchanged review carrier",
	}
}

func openProjectTypeEnvGenesisLedger(
	ctx context.Context,
	mode projectledger.Access,
) (*projectledger.Handle, ProjectBinding, error) {
	binding, err := resolveProjectMemoryAdmissionRoot()
	if err != nil {
		return nil, binding, projectBindingError(binding, err)
	}
	ledger, err := projectledger.OpenExisting(
		ctx,
		binding.ProjectRoot,
		mode,
	)
	if err != nil {
		return nil, binding, fmt.Errorf(
			"open checked Haft project ledger for TypeEnv Genesis: %w; %s",
			err,
			formatProjectBindingDiagnostic(binding),
		)
	}
	if binding.ExpectedProjectID != "" &&
		ledger.ProjectID().String() != binding.ExpectedProjectID {
		closeErr := ledger.Close()
		openErr := fmt.Errorf(
			"open checked Haft project ledger for TypeEnv Genesis: %w: expected %q, bound project is %q",
			errExpectedProjectIDMiss,
			binding.ExpectedProjectID,
			ledger.ProjectID().String(),
		)
		return nil, binding, errors.Join(openErr, closeErr)
	}
	binding.ProjectID = ledger.ProjectID().String()
	return ledger, binding, nil
}

func prepareProjectTypeEnvGenesisReview(
	ctx context.Context,
	ledger *projectledger.Handle,
	baseArtifact typeenv.BaseTypeEnvArtifact,
	clock typedmemorystore.Clock,
) (preparedProjectTypeEnvGenesisReview, error) {
	if len(baseArtifact.CanonicalBytes()) == 0 {
		return preparedProjectTypeEnvGenesisReview{},
			fmt.Errorf("prepare Genesis review: exact base TypeEnv artifact is required")
	}
	service, err := preparationsqlite.NewService(ctx, ledger, clock)
	if err != nil {
		return preparedProjectTypeEnvGenesisReview{}, err
	}
	result, preparationErr := service.PrepareAtBase(ctx, baseArtifact)
	if result == nil {
		return preparedProjectTypeEnvGenesisReview{}, preparationErr
	}
	candidate, resultKind, err := preparedGenesisCandidate(result)
	if err != nil {
		return preparedProjectTypeEnvGenesisReview{},
			errors.Join(preparationErr, err)
	}
	prepared, sealErr := sealProjectTypeEnvGenesisReview(
		candidate,
		resultKind,
		clock.Now(),
	)
	return prepared, errors.Join(preparationErr, sealErr)
}

func preparedGenesisCandidate(
	result preparationsqlite.PreparationResult,
) (projecttypeenvpreparation.GenesisCandidate, string, error) {
	switch value := result.(type) {
	case preparationsqlite.PreparedAtNewBase:
		return value.Candidate(), "prepared_at_new_base", nil
	case preparationsqlite.PreparedAtExistingExactBase:
		return value.Candidate(), "prepared_at_existing_exact_base", nil
	case preparationsqlite.GraphAlreadyActive:
		return projecttypeenvpreparation.GenesisCandidate{},
			"",
			fmt.Errorf(
				"prepare Genesis review: typed-memory graph is already active at revision %d",
				value.Observation().GraphRevision().Value(),
			)
	case preparationsqlite.GraphAdvancedAfterNewBase:
		return projecttypeenvpreparation.GenesisCandidate{},
			"",
			fmt.Errorf(
				"prepare Genesis review: graph advanced to revision %d after base initialization",
				value.Observation().GraphSnapshotBasis().GraphRevision().Value(),
			)
	case preparationsqlite.GraphAdvancedAfterExistingExactBase:
		return projecttypeenvpreparation.GenesisCandidate{},
			"",
			fmt.Errorf(
				"prepare Genesis review: graph advanced to revision %d before candidate persistence",
				value.Observation().GraphSnapshotBasis().GraphRevision().Value(),
			)
	case preparationsqlite.BaseSnapshotConflict:
		return projecttypeenvpreparation.GenesisCandidate{},
			"",
			fmt.Errorf(
				"prepare Genesis review: existing graph base conflicts with bundled FPF TypeEnv",
			)
	default:
		return projecttypeenvpreparation.GenesisCandidate{},
			"",
			fmt.Errorf(
				"prepare Genesis review: unsupported preparation result %T",
				result,
			)
	}
}

func sealProjectTypeEnvGenesisReview(
	candidate projecttypeenvpreparation.GenesisCandidate,
	resultKind string,
	preparedAt time.Time,
) (preparedProjectTypeEnvGenesisReview, error) {
	if err := candidate.Verify(); err != nil {
		return preparedProjectTypeEnvGenesisReview{}, err
	}
	stage := candidate.Stage()
	keyText := "genesis:" + stage.Ref().String()
	key, err :=
		projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
			keyText,
		)
	if err != nil {
		return preparedProjectTypeEnvGenesisReview{}, err
	}
	request, err :=
		projecttypeenvselection.SealGenesisProjectTypeEnvHeadSelectionRequest(
			projecttypeenvselection.GenesisProjectTypeEnvHeadSelectionRequestInput{
				Project:               stage.Project(),
				Stage:                 stage,
				ExpectedGraphRevision: stage.GraphRevision(),
				IdempotencyKey:        key,
			},
		)
	if err != nil {
		return preparedProjectTypeEnvGenesisReview{}, err
	}
	descriptionText := "claim:project-typeenv-genesis:" + stage.Ref().String()
	description, err := authority.NewClaimIDDescriptionRef(descriptionText)
	if err != nil {
		return preparedProjectTypeEnvGenesisReview{}, err
	}
	judgementContext, err := authority.NewBoundedContextRef(
		"bounded-context:haft-project-governance",
	)
	if err != nil {
		return preparedProjectTypeEnvGenesisReview{}, err
	}
	from := preparedAt.Round(0).UTC()
	until := from.Add(projectTypeEnvGenesisReviewWindow)
	validity, err := authority.NewTimeWindow(from, until)
	if err != nil {
		return preparedProjectTypeEnvGenesisReview{}, err
	}
	content, err :=
		projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionAuthorizationContent(
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContentInput{
				DescriptionRef:   description,
				Request:          request,
				Stage:            stage,
				JudgementContext: judgementContext,
				ValidityWindow:   validity,
			},
		)
	if err != nil {
		return preparedProjectTypeEnvGenesisReview{}, err
	}
	response, err := projectTypeEnvGenesisReviewResult(
		stage,
		resultKind,
		validity,
	)
	if err != nil {
		return preparedProjectTypeEnvGenesisReview{}, err
	}
	carrier := projectTypeEnvGenesisReviewCarrier{
		Schema:            projectTypeEnvGenesisReviewSchema,
		ProjectID:         stage.Project().String(),
		PreparationResult: resultKind,
		Review:            response.Review,
		Candidate:         response.Candidate,
		Interpretation:    response.Interpretation,
		StageRef:          stage.Ref().String(),
		RequestRef:        request.Ref().String(),
		RequestDigest:     request.Ref().Digest().String(),
		RequestCanonicalBase64: base64.StdEncoding.EncodeToString(
			request.CanonicalBytes(),
		),
		AuthorizationContentDigest: content.Digest().String(),
		AuthorizationContentCanonicalBase64: base64.StdEncoding.EncodeToString(
			content.CanonicalJSON(),
		),
		PreparedAt: from.Format(time.RFC3339Nano),
		ExpiresAt:  until.Format(time.RFC3339Nano),
	}
	return preparedProjectTypeEnvGenesisReview{
		carrier:  carrier,
		response: response,
	}, nil
}

func projectTypeEnvGenesisReviewResult(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	resultKind string,
	validity authority.TimeWindow,
) (projectTypeEnvGenesisReviewResponse, error) {
	if resultKind != "prepared_at_new_base" &&
		resultKind != "prepared_at_existing_exact_base" {
		return projectTypeEnvGenesisReviewResponse{},
			fmt.Errorf(
				"genesis preparation result %q is unsupported",
				resultKind,
			)
	}
	profilePosture, err := projectTypeEnvProfilePosture(
		stage.ProfileCompatibility(),
	)
	if err != nil {
		return projectTypeEnvGenesisReviewResponse{}, err
	}
	readiness, nextHumanGate := projectTypeEnvGenesisReviewReadinessFor(
		stage,
		profilePosture,
	)
	return projectTypeEnvGenesisReviewResponse{
		ContractVersion: projectTypeEnvGenesisReviewSchema,
		Action:          "prepare",
		Result:          resultKind,
		ProjectID:       stage.Project().String(),
		Review: projectTypeEnvGenesisHumanReview{
			Title:  "Select Haft's first project TypeEnv",
			Choice: "Use the bundled FPF base plus Haft's typed-memory Local-Practice as this project's exact validation, admission, and memory-read ontology",
			WhyNow: "Public EntityOfConcern memory cannot honestly validate, admit, or read project-current semantics until one exact composite is selected",
			Changes: []string{
				"selects one exact composite C as ProjectTypeEnvHead revision 1",
				"activates the already-reviewed typed-memory relation and record mappings",
				"performs and records one bounded CAS Work that changes only the project TypeEnv head and typed-memory graph",
				"makes project-current validation, admission, and exact EntityOfConcern reads resolvable",
			},
			DoesNotChange: []string{
				"project decisions, specifications, source code, or unrelated Work",
				"WorkCommission authority or authorization for additional Work",
				"the bundled FPF source or the prepared immutable B/E/X/C artifacts",
				"release or publication state",
			},
			Validity: projectTypeEnvGenesisReviewValidity{
				From:  validity.From().Format(time.RFC3339Nano),
				Until: validity.Until().Format(time.RFC3339Nano),
			},
			Readiness:       readiness,
			ReturnCondition: "prepare a new review if the project graph, profile, bundled FPF base, Local-Practice source, or installed runtime changes",
		},
		Candidate: projectTypeEnvGenesisCandidateResponse{
			StageRef:                  stage.Ref().String(),
			BaseTypeEnvRef:            stage.Base().String(),
			ExtensionCount:            len(stage.OrderedExtensions()),
			RuntimeEvaluationBasisRef: stage.RuntimeBasis().String(),
			CompositeTypeEnvRef:       stage.VerifiedComposite().String(),
			GraphRevision:             stage.GraphRevision().Value(),
			CompatibilityPosture:      "initial",
			RevalidationPosture:       stage.ExistingAssertionRevalidation().Posture().String(),
			ProfilePosture:            profilePosture,
		},
		Interpretation: projectTypeEnvGenesisReviewInterpretation{
			Establishes: []string{
				"one exact non-binding B/E/X/C/Stage candidate persisted in the project ledger",
				"the exact revision-zero project graph base was initialized or reused",
				"one exact review carrier for later manual selection",
			},
			DoesNotEstablish: []string{
				"ProjectTypeEnvHead selection",
				"typed-memory admission authority",
				"a project-graph U.Work record or authority for additional Work",
				"spec lifecycle approval or release readiness",
			},
			NextHumanGate: nextHumanGate,
		},
	}, nil
}

func projectTypeEnvGenesisReviewReadinessFor(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	profilePosture string,
) (projectTypeEnvGenesisReviewReadiness, string) {
	reasons := make([]string, 0, 2)
	if profilePosture != "compatible" {
		reasons = append(
			reasons,
			"project profile posture is "+profilePosture,
		)
	}
	revalidation := stage.ExistingAssertionRevalidation().Posture()
	if revalidation != typedmemory.RevalidationClean {
		reasons = append(
			reasons,
			"existing assertion revalidation is "+revalidation.String(),
		)
	}
	if len(reasons) > 0 {
		return projectTypeEnvGenesisReviewReadiness{
			Posture: "blocked",
			Reasons: reasons,
			Repair:  "repair or explicitly resolve the project profile and assertion basis, then prepare a fresh exact review",
		}, ""
	}
	return projectTypeEnvGenesisReviewReadiness{
			Posture: "selectable",
			Reasons: []string{},
			Repair:  "",
		},
		"manual h-decide on this exact candidate after the P8 SpecSections are rebaselined"
}

func projectTypeEnvProfilePosture(
	assessment projecttypeenvprofilefit.Assessment,
) (string, error) {
	switch assessment.(type) {
	case projecttypeenvprofilefit.Compatible:
		return "compatible", nil
	case projecttypeenvprofilefit.Incompatible:
		return "incompatible", nil
	case projecttypeenvprofilefit.Underdetermined:
		return "underdetermined", nil
	case projecttypeenvprofilefit.Unavailable:
		return "unavailable", nil
	default:
		return "", fmt.Errorf("project TypeEnv profile posture is invalid")
	}
}

func projectTypeEnvGenesisReviewPath(root string) string {
	return filepath.Join(
		root,
		".haft",
		projectTypeEnvGenesisReviewFileName,
	)
}

func projectTypeEnvGenesisReviewRelativePath() string {
	return filepath.Join(".haft", projectTypeEnvGenesisReviewFileName)
}

func writeProjectTypeEnvGenesisReview(
	projectRoot string,
	carrier projectTypeEnvGenesisReviewCarrier,
) (string, error) {
	return installProjectTypeEnvGenesisReview(projectRoot, carrier, false)
}

func replaceProjectTypeEnvGenesisReview(
	projectRoot string,
	carrier projectTypeEnvGenesisReviewCarrier,
) (string, error) {
	return installProjectTypeEnvGenesisReview(projectRoot, carrier, true)
}

func installProjectTypeEnvGenesisReview(
	projectRoot string,
	carrier projectTypeEnvGenesisReviewCarrier,
	replace bool,
) (string, error) {
	canonical, err := json.MarshalIndent(carrier, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode Genesis review carrier: %w", err)
	}
	canonical = append(canonical, '\n')
	proposed, err := projecttypeenvreviewcarrier.NewCarrier(canonical)
	if err != nil {
		return "", err
	}
	result, err := installProjectTypeEnvGenesisCarrier(
		projectRoot,
		proposed,
		replace,
	)
	if err != nil {
		return "", err
	}
	if unknown, unresolved := result.(projecttypeenvreviewcarrier.OutcomeUnknown); unresolved {
		result, err = retryProjectTypeEnvGenesisCarrier(
			projectRoot,
			proposed,
			unknown,
		)
		if err != nil {
			return "", err
		}
	}
	switch installed := result.(type) {
	case projecttypeenvreviewcarrier.Created:
		return installed.Carrier.Digest().String(), nil
	case projecttypeenvreviewcarrier.Reused:
		return installed.Carrier.Digest().String(), nil
	case projecttypeenvreviewcarrier.Conflict:
		return "", projectTypeEnvGenesisCarrierConflict(installed)
	case projecttypeenvreviewcarrier.OutcomeUnknown:
		return "", fmt.Errorf(
			"genesis review installation may have taken effect, but its final filesystem state could not be verified; inspect %s and select it if present before preparing another review",
			projectTypeEnvGenesisReviewRelativePath(),
		)
	default:
		return "", fmt.Errorf(
			"genesis review installation returned an unsupported result",
		)
	}
}

func readProjectTypeEnvGenesisReview(
	projectRoot string,
) (projectTypeEnvGenesisReviewCarrier, error) {
	observed, err := observeProjectTypeEnvGenesisReview(projectRoot)
	if err != nil {
		return projectTypeEnvGenesisReviewCarrier{}, err
	}
	return observed.carrier, nil
}

func observeProjectTypeEnvGenesisReview(
	projectRoot string,
) (observedProjectTypeEnvGenesisReview, error) {
	sealed, err := projecttypeenvreviewcarrier.Read(projectRoot)
	if err != nil {
		return observedProjectTypeEnvGenesisReview{},
			fmt.Errorf("read Genesis review carrier: %w", err)
	}
	raw := sealed.Bytes()
	carrier := projectTypeEnvGenesisReviewCarrier{}
	if err := decodeStrictGenesisReviewJSON(raw, &carrier); err != nil {
		return observedProjectTypeEnvGenesisReview{}, err
	}
	if carrier.Schema != projectTypeEnvGenesisReviewSchema {
		return observedProjectTypeEnvGenesisReview{},
			fmt.Errorf("genesis review carrier schema %q is unsupported", carrier.Schema)
	}
	ref, err := authority.NewCarrierRef(
		projectTypeEnvGenesisReviewCarrierAuthorityRef,
	)
	if err != nil {
		return observedProjectTypeEnvGenesisReview{},
			fmt.Errorf("construct Genesis review carrier ref: %w", err)
	}
	digest, err := authority.NewDigest(sealed.Digest().String())
	if err != nil {
		return observedProjectTypeEnvGenesisReview{},
			fmt.Errorf("construct Genesis review carrier digest: %w", err)
	}
	binding, err := authority.NewObservableCarrierBinding(ref, digest)
	if err != nil {
		return observedProjectTypeEnvGenesisReview{},
			fmt.Errorf("bind observed Genesis review carrier: %w", err)
	}
	return observedProjectTypeEnvGenesisReview{
		carrier: carrier,
		binding: binding,
	}, nil
}

func installProjectTypeEnvGenesisCarrier(
	projectRoot string,
	proposed projecttypeenvreviewcarrier.Carrier,
	replace bool,
) (projecttypeenvreviewcarrier.InstallationResult, error) {
	if !replace {
		return projecttypeenvreviewcarrier.Install(projectRoot, proposed)
	}
	current, err := projecttypeenvreviewcarrier.Read(projectRoot)
	if err != nil {
		return nil, fmt.Errorf(
			"read current Genesis review before explicit replacement: %w",
			err,
		)
	}
	return projecttypeenvreviewcarrier.Replace(
		projectRoot,
		current.Digest(),
		proposed,
	)
}

func retryProjectTypeEnvGenesisCarrier(
	projectRoot string,
	proposed projecttypeenvreviewcarrier.Carrier,
	unknown projecttypeenvreviewcarrier.OutcomeUnknown,
) (projecttypeenvreviewcarrier.InstallationResult, error) {
	switch retry := unknown.Retry.(type) {
	case projecttypeenvreviewcarrier.ExactInstallRetry:
		return projecttypeenvreviewcarrier.Install(projectRoot, proposed)
	case projecttypeenvreviewcarrier.ExactReplaceRetry:
		return projecttypeenvreviewcarrier.Replace(
			projectRoot,
			retry.Expected,
			proposed,
		)
	default:
		return nil, fmt.Errorf(
			"genesis review installation returned an unsupported retry coordinate",
		)
	}
}

func projectTypeEnvGenesisCarrierConflict(
	conflict projecttypeenvreviewcarrier.Conflict,
) error {
	switch conflict.Expectation.(type) {
	case projecttypeenvreviewcarrier.MustBeAbsent:
		return fmt.Errorf(
			"a different Genesis review carrier already exists; it was retained unchanged; select it or rerun prepare with --replace-review",
		)
	case projecttypeenvreviewcarrier.MustMatch:
		return fmt.Errorf(
			"genesis review changed concurrently; no bytes were replaced; inspect the current review and rerun prepare with --replace-review",
		)
	default:
		return fmt.Errorf(
			"genesis review installation conflict has an unsupported expectation",
		)
	}
}

func decodeProjectTypeEnvGenesisReview(
	ctx context.Context,
	ledger *projectledger.Handle,
	carrier projectTypeEnvGenesisReviewCarrier,
) (
	projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
	projecttypeenvselection.ProjectTypeEnvStage,
	error,
) {
	if carrier.ProjectID != ledger.ProjectID().String() {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			fmt.Errorf("genesis review carrier belongs to another project")
	}
	requestBytes, err := base64.StdEncoding.DecodeString(
		carrier.RequestCanonicalBase64,
	)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			fmt.Errorf("decode Genesis review request: %w", err)
	}
	request, err :=
		projecttypeenvselection.DecodeProjectTypeEnvHeadSelectionRequest(
			requestBytes,
		)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			err
	}
	if request.Project().String() != carrier.ProjectID ||
		request.Ref().String() != carrier.RequestRef ||
		request.Ref().Digest().String() != carrier.RequestDigest ||
		request.Target().Stage().String() != carrier.StageRef {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			fmt.Errorf("genesis review request coordinates do not match carrier")
	}
	store, err := projecttypeenvstage.OpenExisting(
		ctx,
		ledger.Database(),
	)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			err
	}
	ready, err := store.LoadSelectionReady(ctx, request.Target().Stage())
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			err
	}
	stage := ready.Stage()
	contentDigest, err := authority.NewDigest(
		carrier.AuthorizationContentDigest,
	)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			err
	}
	contentCanonical, err := base64.StdEncoding.DecodeString(
		carrier.AuthorizationContentCanonicalBase64,
	)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			fmt.Errorf("decode Genesis authorization content: %w", err)
	}
	content, err :=
		projecttypeenvselectionauthority.DecodeProjectTypeEnvHeadSelectionAuthorizationContent(
			request,
			stage,
			contentCanonical,
			contentDigest,
		)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			err
	}
	preparedAt, err := time.Parse(time.RFC3339Nano, carrier.PreparedAt)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			fmt.Errorf("parse Genesis review preparation time: %w", err)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, carrier.ExpiresAt)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			fmt.Errorf("parse Genesis review expiration time: %w", err)
	}
	validity := content.ValidityWindow()
	if !preparedAt.Equal(validity.From()) ||
		!expiresAt.Equal(validity.Until()) {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			fmt.Errorf("genesis review times differ from authorization content")
	}
	if err := verifyProjectTypeEnvGenesisReviewNarrative(
		stage,
		validity,
		carrier,
	); err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			err
	}
	if carrier.Review.Readiness.Posture != "selectable" {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{},
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{},
			projecttypeenvselection.ProjectTypeEnvStage{},
			fmt.Errorf(
				"genesis review is not selection-ready: %s",
				carrier.Review.Readiness.Repair,
			)
	}
	return request, content, stage, nil
}

func verifyProjectTypeEnvGenesisReviewNarrative(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	validity authority.TimeWindow,
	carrier projectTypeEnvGenesisReviewCarrier,
) error {
	expected, err := projectTypeEnvGenesisReviewResult(
		stage,
		carrier.PreparationResult,
		validity,
	)
	if err != nil {
		return err
	}
	expectedCanonical, err := json.Marshal(struct {
		Review         projectTypeEnvGenesisHumanReview          `json:"review"`
		Candidate      projectTypeEnvGenesisCandidateResponse    `json:"candidate"`
		Interpretation projectTypeEnvGenesisReviewInterpretation `json:"interpretation"`
	}{
		Review:         expected.Review,
		Candidate:      expected.Candidate,
		Interpretation: expected.Interpretation,
	})
	if err != nil {
		return fmt.Errorf("encode expected Genesis review narrative: %w", err)
	}
	actualCanonical, err := json.Marshal(struct {
		Review         projectTypeEnvGenesisHumanReview          `json:"review"`
		Candidate      projectTypeEnvGenesisCandidateResponse    `json:"candidate"`
		Interpretation projectTypeEnvGenesisReviewInterpretation `json:"interpretation"`
	}{
		Review:         carrier.Review,
		Candidate:      carrier.Candidate,
		Interpretation: carrier.Interpretation,
	})
	if err != nil {
		return fmt.Errorf("encode Genesis review carrier narrative: %w", err)
	}
	if !bytes.Equal(expectedCanonical, actualCanonical) {
		return fmt.Errorf(
			"genesis review narrative differs from the exact prepared Stage",
		)
	}
	return nil
}

func decodeStrictGenesisReviewJSON(
	raw []byte,
	target any,
) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode Genesis review carrier: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("genesis review carrier has trailing material")
	}
	return nil
}

func projectTypeEnvGenesisResultResponse(
	projectID string,
	result projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionResult,
) (projectTypeEnvGenesisSelectionResponse, error) {
	response := projectTypeEnvGenesisSelectionResponse{
		ContractVersion: "haft.project-typeenv.genesis-selection/v1",
		Action:          "select",
		ProjectID:       projectID,
		Interpretation: projectTypeEnvGenesisSelectionInterpretation{
			Establishes:      []string{},
			DoesNotEstablish: []string{},
			DoesNotAuthorize: []string{
				"binding project decisions",
				"WorkCommission creation",
				"spec lifecycle changes",
				"release or publication",
			},
		},
	}
	switch value := result.(type) {
	case projecttypeenvselectioneffect.FreshlyCommitted:
		delivery, err := genesisDeliveryPosture(value.Delivery())
		if err != nil {
			return projectTypeEnvGenesisSelectionResponse{}, err
		}
		response.Outcome = projectTypeEnvGenesisFreshlyCommitted{
			Kind:             "freshly_committed",
			DeliveryPosture:  delivery,
			CommittedClosure: genesisCommittedClosureWire(value.Closure()),
		}
		response.Interpretation.Establishes = []string{
			"the exact selected project TypeEnv composite and head revision",
			"one performed bounded CAS Work plus its exact authority-use, receipt, and replay closure",
		}
		return response, nil
	case projecttypeenvselectioneffect.ReplayedExisting:
		response.Outcome = projectTypeEnvGenesisReplayedExisting{
			Kind:             "replayed_existing",
			CommittedClosure: genesisCommittedClosureWire(value.Closure()),
		}
		response.Interpretation.Establishes = []string{
			"the exact previously selected project TypeEnv composite and head revision",
			"the exact previously committed authority-use, CAS Work, receipt, and replay closure",
		}
		response.Interpretation.DoesNotEstablish = []string{
			"a new semantic graph write or Work occurrence from this invocation",
			"current Stage, profile, authority, or head revalidation",
		}
		return response, nil
	case projecttypeenvselectioneffect.NotSelected:
		response.Outcome = projectTypeEnvGenesisNotSelected{
			Kind:   "not_selected",
			Reason: value.Reason().String(),
			Repair: genesisSelectionRepair(value.Reason()),
		}
		response.Interpretation.Establishes = []string{
			"this selection attempt did not accept the requested ProjectTypeEnvHead change",
		}
		response.Interpretation.DoesNotEstablish = []string{
			"a selected head, receipt, or performed head-selection Work",
		}
		return response, nil
	case projecttypeenvselectioneffect.ReplayConflict:
		response.Outcome = projectTypeEnvGenesisReplayConflict{
			Kind:                   "replay_conflict",
			IdempotencyKey:         value.Key().String(),
			ExistingRequestDigest:  value.ExistingRequestDigest().String(),
			PresentedRequestDigest: value.PresentedRequestDigest().String(),
			ExistingContentDigest:  value.ExistingContentDigest().String(),
			PresentedContentDigest: value.PresentedContentDigest().String(),
			Repair:                 "inspect the existing attempt; do not overwrite it or reuse this key for changed content",
		}
		response.Interpretation.Establishes = []string{
			"the idempotency key is already bound to a different exact selection request",
		}
		response.Interpretation.DoesNotEstablish = []string{
			"a new head selection, graph write, receipt, or rollback",
		}
		return response, nil
	case projecttypeenvselectioneffect.CommitOutcomeUnknown:
		response.Outcome = projectTypeEnvGenesisCommitOutcomeUnknown{
			Kind:          "commit_outcome_unknown",
			RetryKey:      value.RetryKey().String(),
			RequestDigest: value.RequestDigest().String(),
			ContentDigest: value.ContentDigest().String(),
			Repair:        "retry the unchanged key, request, and authorization content; exact replay recovers a prior commit, otherwise current checks govern a fresh attempt",
		}
		response.Interpretation.Establishes = []string{
			"the caller cannot yet determine whether the exact selection committed",
		}
		response.Interpretation.DoesNotEstablish = []string{
			"commit or rollback",
			"a selected head, receipt, or no-write conclusion",
		}
		return response, nil
	default:
		return projectTypeEnvGenesisSelectionResponse{},
			fmt.Errorf("unsupported Genesis selection result %T", result)
	}
}

func genesisCommittedClosureWire(
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) projectTypeEnvGenesisCommittedClosureWire {
	head := closure.SuccessorHead()
	return projectTypeEnvGenesisCommittedClosureWire{
		ClosureRef:             closure.Ref().String(),
		ClosureDigest:          closure.Digest().String(),
		TransactionRef:         closure.TransactionRef().String(),
		TransactionDigest:      closure.TransactionDigest().String(),
		IdempotencyKey:         closure.IdempotencyKey().String(),
		RequestRef:             closure.RequestRef().String(),
		RequestDigest:          closure.RequestDigest().String(),
		AuthorityBasisKind:     closure.AuthorityCoordinates().Kind().String(),
		SelectedCompositeRef:   closure.Target().Composite().String(),
		HeadRef:                head.Ref().String(),
		HeadRevision:           head.Revision().Value(),
		ExpectedGraphRevision:  closure.ExpectedGraphRevision().Value(),
		CommittedGraphRevision: closure.CommittedGraphRevision().Value(),
		AuthorityUseRecordRef:  closure.AuthorityUseRecordRef().String(),
		WorkRef:                closure.WorkRef().String(),
		CASWorkRecordRef:       closure.CASWorkRecordRef().String(),
		GraphEventRef:          closure.EventRef().String(),
		GraphCommitRef:         closure.CommitRef().String(),
		ReceiptRef:             closure.ReceiptRef().String(),
		ReceiptDigest:          closure.ReceiptDigest().String(),
		CommittedResultRef:     closure.CommittedResultRef().String(),
		CommittedResultDigest:  closure.CommittedResultDigest().String(),
	}
}

func genesisDeliveryPosture(
	posture projecttypeenvselectioneffect.SuccessfulProjectTypeEnvHeadSelectionDeliveryPosture,
) (string, error) {
	switch posture.(type) {
	case projecttypeenvselectioneffect.CommittedAndObserved:
		return "committed_and_observed", nil
	case projecttypeenvselectioneffect.CommitRecoveredByExactClosureReread:
		return "commit_recovered_by_exact_closure_reread", nil
	default:
		return "", fmt.Errorf(
			"unsupported successful Genesis delivery posture %T",
			posture,
		)
	}
}

func genesisSelectionRepair(
	reason projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason,
) string {
	switch reason.String() {
	case "current_authority_rejection":
		return "invoke h-decide manually for this exact review; strict_cli_speech_act projects additionally require the reviewed terminal SpeechAct"
	case "review_expired":
		return "prepare a fresh exact review carrier; another h-decide cannot revive expired authorization content"
	case "profile_incompatible", "profile_underdetermined", "profile_drift":
		return "repair or explicitly review the current project-profile basis, then prepare a new exact carrier"
	case "stale_graph", "stage_drift", "assertion_revalidation_failure":
		return "re-read the project graph and prepare a new exact carrier; do not reuse the stale Stage"
	case "prior_head_exists":
		return "use the post-Genesis transition path against the exact current head"
	default:
		return "inspect the exact rejection basis before preparing or selecting another candidate"
	}
}
