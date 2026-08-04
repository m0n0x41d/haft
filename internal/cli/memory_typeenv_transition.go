package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvreviewcarrier"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	selectionsqlite "github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/projecttypeenvtransitionpreparation"
	transitionsqlite "github.com/m0n0x41d/haft/internal/projecttypeenvtransitionpreparation/sqlite"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/spf13/cobra"
)

const projectTypeEnvTransitionReviewSchema = "haft.project-typeenv.transition-review/v1"

type projectTypeEnvTransitionReviewCarrier struct {
	Schema                              string                                    `json:"schema"`
	ProjectID                           string                                    `json:"project_id"`
	PreparationResult                   string                                    `json:"preparation_result"`
	Review                              projectTypeEnvTransitionHumanReview       `json:"review"`
	Candidate                           projectTypeEnvTransitionCandidateResponse `json:"candidate"`
	Interpretation                      projectTypeEnvGenesisReviewInterpretation `json:"interpretation"`
	StageRef                            string                                    `json:"stage_ref"`
	RequestRef                          string                                    `json:"request_ref"`
	RequestDigest                       string                                    `json:"request_digest"`
	RequestCanonicalBase64              string                                    `json:"request_canonical_base64"`
	AuthorizationContentDigest          string                                    `json:"authorization_content_digest"`
	AuthorizationContentCanonicalBase64 string                                    `json:"authorization_content_canonical_base64"`
	TransitionReviewArtifactDigest      string                                    `json:"transition_review_artifact_digest"`
	TransitionReviewArtifactBase64      string                                    `json:"transition_review_artifact_base64"`
	PreparedAt                          string                                    `json:"prepared_at"`
	ExpiresAt                           string                                    `json:"expires_at"`
}

type projectTypeEnvTransitionHumanReview struct {
	Title           string                                  `json:"title"`
	Choice          string                                  `json:"choice"`
	WhyNow          string                                  `json:"why_now"`
	Changes         []string                                `json:"changes"`
	DoesNotChange   []string                                `json:"does_not_change"`
	Validity        projectTypeEnvGenesisReviewValidity     `json:"validity"`
	Readiness       projectTypeEnvTransitionReviewReadiness `json:"readiness"`
	ReturnCondition string                                  `json:"return_condition"`
}

type projectTypeEnvTransitionReviewReadiness struct {
	Posture  string   `json:"posture"`
	Reasons  []string `json:"reasons"`
	Warnings []string `json:"warnings"`
	Repair   string   `json:"repair"`
}

type projectTypeEnvTransitionCandidateResponse struct {
	StageRef                  string                                         `json:"stage_ref"`
	PriorHeadRevision         uint64                                         `json:"prior_head_revision"`
	PriorCompositeTypeEnvRef  string                                         `json:"prior_composite_type_env_ref"`
	BaseTypeEnvRef            string                                         `json:"base_type_env_ref"`
	ExtensionCount            int                                            `json:"extension_count"`
	RuntimeEvaluationBasisRef string                                         `json:"runtime_evaluation_basis_ref"`
	CompositeTypeEnvRef       string                                         `json:"composite_type_env_ref"`
	GraphRevision             uint64                                         `json:"graph_revision"`
	Compatibility             projectTypeEnvTransitionCompatibilitySummary   `json:"compatibility"`
	RevalidationPosture       string                                         `json:"revalidation_posture"`
	ProjectProfilePosture     string                                         `json:"project_profile_posture"`
	ProjectionProfiles        []projectTypeEnvTransitionProfileCompatibility `json:"projection_profiles"`
}

type projectTypeEnvTransitionCompatibilitySummary struct {
	Unchanged   int `json:"unchanged"`
	Additive    int `json:"additive"`
	Widened     int `json:"widened"`
	Narrowed    int `json:"narrowed"`
	Removed     int `json:"removed"`
	CompilerGap int `json:"compiler_gap"`
}

type projectTypeEnvTransitionProfileCompatibility struct {
	ProfileRef     string   `json:"profile_ref"`
	ProfileEdition uint32   `json:"profile_edition"`
	Posture        string   `json:"posture"`
	AffectedFacets []string `json:"affected_facets"`
}

type projectTypeEnvTransitionReviewResponse struct {
	ContractVersion               string                                      `json:"contract_version"`
	Action                        string                                      `json:"action"`
	Result                        string                                      `json:"result"`
	ProjectID                     string                                      `json:"project_id"`
	Review                        projectTypeEnvTransitionHumanReview         `json:"review"`
	Candidate                     projectTypeEnvTransitionCandidateResponse   `json:"candidate"`
	ReviewCarrier                 projectTypeEnvGenesisReviewCarrierRef       `json:"review_carrier"`
	PostPrepareLedgerRevalidation projectTypeEnvGenesisPostEffectRevalidation `json:"post_prepare_ledger_revalidation"`
	Interpretation                projectTypeEnvGenesisReviewInterpretation   `json:"interpretation"`
}

type projectTypeEnvTransitionAlreadySelectedResponse struct {
	ContractVersion string                                    `json:"contract_version"`
	Action          string                                    `json:"action"`
	Result          string                                    `json:"result"`
	ProjectID       string                                    `json:"project_id"`
	Current         projectTypeEnvTransitionAlreadySelected   `json:"current"`
	Interpretation  projectTypeEnvGenesisReviewInterpretation `json:"interpretation"`
}

type projectTypeEnvTransitionAlreadySelected struct {
	HeadRevision     uint64 `json:"head_revision"`
	CompositeTypeEnv string `json:"composite_type_env_ref"`
}

type preparedProjectTypeEnvTransitionReview struct {
	carrier  projectTypeEnvTransitionReviewCarrier
	response projectTypeEnvTransitionReviewResponse
}

type observedProjectTypeEnvTransitionReview struct {
	carrier projectTypeEnvTransitionReviewCarrier
	binding authority.ObservableCarrierBinding
}

type projectTypeEnvTransitionSelectionInputs struct {
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent
}

func tryRunMemoryTypeEnvTransitionPrepare(
	cmd *cobra.Command,
	ledger *projectledger.Handle,
	binding ProjectBinding,
	base typeenv.BaseTypeEnvArtifact,
	clock typedmemorystore.Clock,
) (bool, error) {
	service, err := transitionsqlite.NewService(cmd.Context(), ledger)
	if err != nil {
		return true, err
	}
	result, preparationErr := service.PrepareAtBase(cmd.Context(), base)
	switch value := result.(type) {
	case transitionsqlite.NoPriorHead:
		return false, preparationErr
	case transitionsqlite.AlreadySelected:
		response := projectTypeEnvTransitionAlreadySelectedResponse{
			ContractVersion: projectTypeEnvTransitionReviewSchema,
			Action:          "prepare",
			Result:          "already_selected",
			ProjectID:       ledger.ProjectID().String(),
			Current: projectTypeEnvTransitionAlreadySelected{
				HeadRevision:     value.Head().Revision().Value(),
				CompositeTypeEnv: value.Head().SelectedComposite().String(),
			},
			Interpretation: projectTypeEnvGenesisReviewInterpretation{
				Establishes: []string{
					"the bundled TypeEnv target already equals the exact current project head",
				},
				DoesNotEstablish: []string{
					"a new Stage, review, authority act, head revision, graph revision, or Work occurrence",
				},
				NextHumanGate: "",
			},
		}
		writeErr := writeJSON(cmd.OutOrStdout(), response)
		return true, errors.Join(preparationErr, writeErr)
	case transitionsqlite.Prepared:
		prepared, sealErr := sealProjectTypeEnvTransitionReview(
			value.Candidate(),
			clock.Now(),
		)
		if sealErr != nil {
			return true, errors.Join(preparationErr, sealErr)
		}
		if preparationErr != nil {
			response := prepared.response
			response.Result = "prepared_but_current_basis_changed"
			response.Review.Readiness = projectTypeEnvTransitionReviewReadiness{
				Posture: "blocked",
				Reasons: []string{
					"the project head, graph, profile, or root attachment changed after durable preparation",
				},
				Warnings: []string{},
				Repair:   "inspect the retained immutable candidate and prepare a fresh review against the current project basis",
			}
			response.Interpretation.NextHumanGate = ""
			response.Interpretation.DoesNotEstablish = append(
				response.Interpretation.DoesNotEstablish,
				"the currentness of the prepared predecessor, graph, profile, or project attachment",
			)
			response.PostPrepareLedgerRevalidation = genesisLedgerRevalidationResult(preparationErr)
			writeErr := writeJSON(cmd.OutOrStdout(), response)
			return true, errors.Join(preparationErr, writeErr)
		}
		if prepared.response.Review.Readiness.Posture == "selectable" {
			response, selectionErr := executeAutomaticCompatibleTransition(
				cmd.Context(),
				ledger,
				value.Candidate(),
				clock,
			)
			var cleanupErr error
			if selectionErr == nil && defaultMemorySelectionCommitted(response) {
				cleanupErr = removeConsumedDefaultMemoryReview(binding.ProjectRoot)
			}
			writeErr := writeJSON(cmd.OutOrStdout(), response)
			return true, errors.Join(selectionErr, cleanupErr, writeErr)
		}
		digest, installErr := installProjectTypeEnvTransitionReview(
			binding.ProjectRoot,
			prepared.carrier,
			memoryTypeEnvPrepareReplaceReview,
		)
		if installErr != nil {
			return true, installErr
		}
		response := prepared.response
		response.ReviewCarrier = projectTypeEnvGenesisReviewCarrierRef{
			Path:   projectTypeEnvGenesisReviewRelativePath(),
			Digest: digest,
		}
		revalidationErr := ledger.Revalidate(cmd.Context())
		response.PostPrepareLedgerRevalidation = genesisLedgerRevalidationResult(revalidationErr)
		writeErr := writeJSON(cmd.OutOrStdout(), response)
		if revalidationErr == nil {
			return true, writeErr
		}
		return true, errors.Join(
			fmt.Errorf("revalidate project after writing Transition review: %w", revalidationErr),
			writeErr,
		)
	case nil:
		return true, preparationErr
	default:
		return true, fmt.Errorf("transition preparation returned an unsupported result %T", result)
	}
}

func sealProjectTypeEnvTransitionReview(
	candidate projecttypeenvtransitionpreparation.Candidate,
	preparedAt time.Time,
) (preparedProjectTypeEnvTransitionReview, error) {
	inputs, err := sealProjectTypeEnvTransitionSelectionInputs(candidate, preparedAt)
	if err != nil {
		return preparedProjectTypeEnvTransitionReview{}, err
	}
	request := inputs.request
	content := inputs.content
	stage := candidate.Stage()
	validity := content.ValidityWindow()
	from := validity.From()
	until := validity.Until()
	response, err := projectTypeEnvTransitionReviewResult(candidate, validity)
	if err != nil {
		return preparedProjectTypeEnvTransitionReview{}, err
	}
	profiles := candidate.TransitionProjectionProfiles()
	carrier := projectTypeEnvTransitionReviewCarrier{
		Schema:            projectTypeEnvTransitionReviewSchema,
		ProjectID:         stage.Project().String(),
		PreparationResult: "prepared_successor",
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
		TransitionReviewArtifactDigest: profiles.Digest().String(),
		TransitionReviewArtifactBase64: base64.StdEncoding.EncodeToString(
			profiles.CanonicalBytes(),
		),
		PreparedAt: from.Format(time.RFC3339Nano),
		ExpiresAt:  until.Format(time.RFC3339Nano),
	}
	return preparedProjectTypeEnvTransitionReview{
		carrier:  carrier,
		response: response,
	}, nil
}

func sealProjectTypeEnvTransitionSelectionInputs(
	candidate projecttypeenvtransitionpreparation.Candidate,
	preparedAt time.Time,
) (projectTypeEnvTransitionSelectionInputs, error) {
	if err := candidate.Verify(); err != nil {
		return projectTypeEnvTransitionSelectionInputs{}, err
	}
	stage := candidate.Stage()
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		"transition:" + stage.Ref().String(),
	)
	if err != nil {
		return projectTypeEnvTransitionSelectionInputs{}, err
	}
	request, err := projecttypeenvselection.SealTransitionProjectTypeEnvHeadSelectionRequest(
		projecttypeenvselection.TransitionProjectTypeEnvHeadSelectionRequestInput{
			Project:               stage.Project(),
			ExactPriorHead:        candidate.PriorHead(),
			Stage:                 stage,
			ExpectedGraphRevision: stage.GraphRevision(),
			IdempotencyKey:        key,
		},
	)
	if err != nil {
		return projectTypeEnvTransitionSelectionInputs{}, err
	}
	description, err := authority.NewClaimIDDescriptionRef(
		"claim:project-typeenv-transition:" + stage.Ref().String(),
	)
	if err != nil {
		return projectTypeEnvTransitionSelectionInputs{}, err
	}
	judgementContext, err := authority.NewBoundedContextRef(
		"bounded-context:haft-project-governance",
	)
	if err != nil {
		return projectTypeEnvTransitionSelectionInputs{}, err
	}
	from := preparedAt.Round(0).UTC()
	until := from.Add(projectTypeEnvGenesisReviewWindow)
	validity, err := authority.NewTimeWindow(from, until)
	if err != nil {
		return projectTypeEnvTransitionSelectionInputs{}, err
	}
	content, err := projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionAuthorizationContent(
		projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContentInput{
			DescriptionRef:   description,
			Request:          request,
			Stage:            stage,
			JudgementContext: judgementContext,
			ValidityWindow:   validity,
		},
	)
	if err != nil {
		return projectTypeEnvTransitionSelectionInputs{}, err
	}
	return projectTypeEnvTransitionSelectionInputs{
		request: request,
		content: content,
	}, nil
}

func projectTypeEnvTransitionReviewResult(
	candidate projecttypeenvtransitionpreparation.Candidate,
	validity authority.TimeWindow,
) (projectTypeEnvTransitionReviewResponse, error) {
	stage := candidate.Stage()
	projectProfilePosture, err := projectTypeEnvProfilePosture(stage.ProfileCompatibility())
	if err != nil {
		return projectTypeEnvTransitionReviewResponse{}, err
	}
	profiles := candidate.ProjectionProfiles()
	readiness, nextHumanGate := projectTypeEnvTransitionReadiness(
		stage,
		projectProfilePosture,
		profiles,
	)
	return projectTypeEnvTransitionReviewResponse{
		ContractVersion: projectTypeEnvTransitionReviewSchema,
		Action:          "prepare",
		Result:          "prepared_successor",
		ProjectID:       stage.Project().String(),
		Review: projectTypeEnvTransitionHumanReview{
			Title:  "Select the reviewed project TypeEnv successor",
			Choice: "Move this project's exact validation, admission, and memory-read ontology from the current composite to the bundled successor",
			WhyNow: "The bundled FPF and Haft Local-Practice target differs from the exact project-selected TypeEnv; shipping it alone does not change project memory semantics",
			Changes: []string{
				"advances ProjectTypeEnvHead by one revision from the exact reviewed predecessor",
				"selects the reviewed successor composite for future validation, admission, and typed reads",
				"records one bounded CAS Work and advances the typed-memory graph by one revision",
				"invalidates successor-sensitive read caches while preserving old snapshot coordinates",
			},
			DoesNotChange: []string{
				"historical assertions or their original TypeEnv basis",
				"project decisions, specifications, source code, or unrelated Work",
				"immutable prior B/E/X/C, Stage, profile, or receipt bytes",
				"release, publication, WorkCommission, or specification authority",
			},
			Validity: projectTypeEnvGenesisReviewValidity{
				From:  validity.From().Format(time.RFC3339Nano),
				Until: validity.Until().Format(time.RFC3339Nano),
			},
			Readiness:       readiness,
			ReturnCondition: "prepare a fresh review if the head, graph, profile catalog, project profile, bundled FPF base, Local-Practice source, or installed runtime changes",
		},
		Candidate: projectTypeEnvTransitionCandidateResponse{
			StageRef:                  stage.Ref().String(),
			PriorHeadRevision:         candidate.PriorHead().Revision().Value(),
			PriorCompositeTypeEnvRef:  candidate.PriorHead().SelectedComposite().String(),
			BaseTypeEnvRef:            stage.Base().String(),
			ExtensionCount:            len(stage.OrderedExtensions()),
			RuntimeEvaluationBasisRef: stage.RuntimeBasis().String(),
			CompositeTypeEnvRef:       stage.VerifiedComposite().String(),
			GraphRevision:             stage.GraphRevision().Value(),
			Compatibility:             transitionCompatibilitySummary(candidate.SuccessorDiff()),
			RevalidationPosture:       stage.ExistingAssertionRevalidation().Posture().String(),
			ProjectProfilePosture:     projectProfilePosture,
			ProjectionProfiles:        transitionProfileResponses(profiles),
		},
		Interpretation: projectTypeEnvGenesisReviewInterpretation{
			Establishes: []string{
				"one exact non-binding successor B/E/X/C/Stage candidate persisted in the project ledger",
				"one exact compatibility and revalidation review against the current head, graph, and profiles",
				"one exact review carrier for later manual selection",
			},
			DoesNotEstablish: []string{
				"ProjectTypeEnvHead selection",
				"typed-memory admission authority under the successor",
				"migration or reinterpretation of historical values and assertions",
				"spec lifecycle approval, release readiness, or authority for other Work",
			},
			NextHumanGate: nextHumanGate,
		},
	}, nil
}

func projectTypeEnvTransitionReadiness(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	projectProfilePosture string,
	profiles projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet,
) (projectTypeEnvTransitionReviewReadiness, string) {
	reasons := make([]string, 0, 3)
	warnings := make([]string, 0)
	if projectProfilePosture != "compatible" {
		reasons = append(reasons, "project declaration profile posture is "+projectProfilePosture)
	}
	revalidation := stage.ExistingAssertionRevalidation().Posture()
	if revalidation != typedmemory.RevalidationClean {
		reasons = append(reasons, "existing assertion revalidation is "+revalidation.String())
	}
	if profiles.HasBlockedProfile() {
		reasons = append(reasons, "at least one installed projection profile is blocked by the successor")
	}
	for _, profile := range profiles.Profiles() {
		if profile.Kind() == projecttypeenvprofilecompatibility.ProfileDegradedFacets {
			warnings = append(
				warnings,
				"projection profile "+profile.ProfileRef().String()+" has degraded facets",
			)
		}
	}
	if len(reasons) != 0 {
		return projectTypeEnvTransitionReviewReadiness{
			Posture:  "blocked",
			Reasons:  reasons,
			Warnings: warnings,
			Repair:   "repair the named assertion, project-profile, or projection-profile basis, then prepare a fresh successor review",
		}, ""
	}
	return projectTypeEnvTransitionReviewReadiness{
		Posture:  "selectable",
		Reasons:  []string{},
		Warnings: warnings,
		Repair:   "",
	}, "direct unambiguous operator selection of this exact readable successor review"
}

func transitionCompatibilitySummary(
	diff projecttypeenvcompatibility.SuccessorDiff,
) projectTypeEnvTransitionCompatibilitySummary {
	result := projectTypeEnvTransitionCompatibilitySummary{}
	for _, rule := range diff.Rules() {
		switch rule.Class() {
		case projecttypeenvcompatibility.SuccessorUnchanged:
			result.Unchanged++
		case projecttypeenvcompatibility.SuccessorAdditive:
			result.Additive++
		case projecttypeenvcompatibility.SuccessorWidened:
			result.Widened++
		case projecttypeenvcompatibility.SuccessorNarrowed:
			result.Narrowed++
		case projecttypeenvcompatibility.SuccessorRemoved:
			result.Removed++
		case projecttypeenvcompatibility.SuccessorCompilerGap:
			result.CompilerGap++
		}
	}
	return result
}

func transitionProfileResponses(
	set projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet,
) []projectTypeEnvTransitionProfileCompatibility {
	profiles := set.Profiles()
	result := make([]projectTypeEnvTransitionProfileCompatibility, 0, len(profiles))
	for _, profile := range profiles {
		facets := profile.AffectedFacets()
		affected := make([]string, 0, len(facets))
		for _, facet := range facets {
			affected = append(affected, string(facet))
		}
		result = append(result, projectTypeEnvTransitionProfileCompatibility{
			ProfileRef:     profile.ProfileRef().String(),
			ProfileEdition: profile.ProfileEdition(),
			Posture:        profile.Kind().String(),
			AffectedFacets: affected,
		})
	}
	return result
}

func installProjectTypeEnvTransitionReview(
	projectRoot string,
	carrier projectTypeEnvTransitionReviewCarrier,
	replace bool,
) (string, error) {
	canonical, err := json.MarshalIndent(carrier, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode Transition review carrier: %w", err)
	}
	canonical = append(canonical, '\n')
	proposed, err := projecttypeenvreviewcarrier.NewCarrier(canonical)
	if err != nil {
		return "", err
	}
	result, err := installProjectTypeEnvGenesisCarrier(projectRoot, proposed, replace)
	if err != nil {
		return "", err
	}
	if unknown, unresolved := result.(projecttypeenvreviewcarrier.OutcomeUnknown); unresolved {
		result, err = retryProjectTypeEnvGenesisCarrier(projectRoot, proposed, unknown)
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
		return "", projectTypeEnvTransitionCarrierConflict(installed)
	case projecttypeenvreviewcarrier.OutcomeUnknown:
		return "", fmt.Errorf("transition review installation may have taken effect; inspect the known review carrier and retry select before preparing another review")
	default:
		return "", fmt.Errorf("transition review installation returned an unsupported result")
	}
}

func projectTypeEnvTransitionCarrierConflict(
	conflict projecttypeenvreviewcarrier.Conflict,
) error {
	switch conflict.Expectation.(type) {
	case projecttypeenvreviewcarrier.MustBeAbsent:
		return fmt.Errorf("a different TypeEnv review already exists and was retained unchanged; select it or rerun prepare with --replace-review")
	case projecttypeenvreviewcarrier.MustMatch:
		return fmt.Errorf("the TypeEnv review changed concurrently; no bytes were replaced; inspect it and rerun prepare with --replace-review")
	default:
		return fmt.Errorf("transition review installation conflict has an unsupported expectation")
	}
}

func projectTypeEnvReviewSchema(projectRoot string) (string, error) {
	sealed, err := projecttypeenvreviewcarrier.Read(projectRoot)
	if err != nil {
		return "", err
	}
	envelope := struct {
		Schema string `json:"schema"`
	}{}
	if err := json.Unmarshal(sealed.Bytes(), &envelope); err != nil {
		return "", fmt.Errorf("decode TypeEnv review schema: %w", err)
	}
	if envelope.Schema == "" {
		return "", fmt.Errorf("TypeEnv review schema is required")
	}
	return envelope.Schema, nil
}

func executeMemoryTypeEnvTransitionSelection(
	ctx context.Context,
	ledger *projectledger.Handle,
	binding ProjectBinding,
) (projectTypeEnvGenesisSelectionResponse, error) {
	observed, err := observeProjectTypeEnvTransitionReview(binding.ProjectRoot)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	request, content, _, target, err := decodeProjectTypeEnvTransitionReview(
		ctx,
		ledger,
		observed.carrier,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	service, err := selectionsqlite.NewTransitionService(
		ctx,
		ledger.Database(),
		binding.ProjectRoot,
		target.InstalledRuntime(),
		typedmemorystore.SystemClock{},
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	payload, err := projecttypeenvselectionauthority.HostRoutedSelectionPayload(
		request,
		content,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	operatorRequest, err := operatorrequest.New(
		operatorrequest.ProjectTypeEnvHeadSelect,
		request.Ref().String(),
		payload,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	ingress, err := selectionsqlite.NewHostRoutedOperatorRequest(operatorRequest)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	result, err := service.SelectTransition(
		ctx,
		selectionsqlite.TransitionSelectionInput{
			Request:   request,
			Content:   content,
			Authority: ingress,
		},
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	response, err := projectTypeEnvTransitionResultResponse(
		ledger.ProjectID().String(),
		result,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	response.AuthorityIngress = string(operatorRequest.Provenance())
	revalidationErr := ledger.Revalidate(ctx)
	response.PostEffectLedgerRevalidation = genesisLedgerRevalidationResult(revalidationErr)
	if revalidationErr == nil {
		return response, nil
	}
	return response, fmt.Errorf(
		"revalidate project after Transition selection: %w",
		revalidationErr,
	)
}

// executeAutomaticCompatibleTransition applies the package-owned successor
// policy directly to one freshly prepared candidate. It creates no operator
// request and consumes no human review carrier. The effect shell repeats the
// compatibility and currentness checks inside the head CAS transaction.
func executeAutomaticCompatibleTransition(
	ctx context.Context,
	ledger *projectledger.Handle,
	candidate projecttypeenvtransitionpreparation.Candidate,
	clock typedmemorystore.Clock,
) (projectTypeEnvGenesisSelectionResponse, error) {
	inputs, err := sealProjectTypeEnvTransitionSelectionInputs(
		candidate,
		clock.Now(),
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	service, err := selectionsqlite.NewTransitionService(
		ctx,
		ledger.Database(),
		ledger.ProjectRoot().String(),
		candidate.Target().InstalledRuntime(),
		clock,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	result, err := service.SelectTransition(
		ctx,
		selectionsqlite.TransitionSelectionInput{
			Request:   inputs.request,
			Content:   inputs.content,
			Authority: selectionsqlite.NewAutomaticCompatibleSuccessorIngress(),
		},
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	response, err := projectTypeEnvTransitionResultResponse(
		ledger.ProjectID().String(),
		result,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	response.AuthorityIngress =
		projecttypeenvselectionauthority.CompatibleSuccessorAuthorityGeneration
	revalidationErr := ledger.Revalidate(ctx)
	response.PostEffectLedgerRevalidation =
		genesisLedgerRevalidationResult(revalidationErr)
	if revalidationErr == nil {
		return response, nil
	}
	return response, fmt.Errorf(
		"revalidate project after automatic compatible Transition: %w",
		revalidationErr,
	)
}

func observeProjectTypeEnvTransitionReview(
	projectRoot string,
) (observedProjectTypeEnvTransitionReview, error) {
	sealed, err := projecttypeenvreviewcarrier.Read(projectRoot)
	if err != nil {
		return observedProjectTypeEnvTransitionReview{},
			fmt.Errorf("read Transition review carrier: %w", err)
	}
	carrier := projectTypeEnvTransitionReviewCarrier{}
	if err := decodeStrictTransitionReviewJSON(sealed.Bytes(), &carrier); err != nil {
		return observedProjectTypeEnvTransitionReview{}, err
	}
	if carrier.Schema != projectTypeEnvTransitionReviewSchema {
		return observedProjectTypeEnvTransitionReview{},
			fmt.Errorf("transition review carrier schema %q is unsupported", carrier.Schema)
	}
	ref, err := authority.NewCarrierRef(projectTypeEnvGenesisReviewCarrierAuthorityRef)
	if err != nil {
		return observedProjectTypeEnvTransitionReview{}, err
	}
	digest, err := authority.NewDigest(sealed.Digest().String())
	if err != nil {
		return observedProjectTypeEnvTransitionReview{}, err
	}
	carrierBinding, err := authority.NewObservableCarrierBinding(ref, digest)
	if err != nil {
		return observedProjectTypeEnvTransitionReview{}, err
	}
	return observedProjectTypeEnvTransitionReview{
		carrier: carrier,
		binding: carrierBinding,
	}, nil
}

func decodeProjectTypeEnvTransitionReview(
	ctx context.Context,
	ledger *projectledger.Handle,
	carrier projectTypeEnvTransitionReviewCarrier,
) (
	projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
	projecttypeenvselection.ProjectTypeEnvStage,
	localpracticeruntime.Target,
	error,
) {
	emptyRequest := projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{}
	emptyContent := projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{}
	emptyStage := projecttypeenvselection.ProjectTypeEnvStage{}
	emptyTarget := localpracticeruntime.Target{}
	if carrier.ProjectID != ledger.ProjectID().String() {
		return emptyRequest, emptyContent, emptyStage, emptyTarget,
			fmt.Errorf("transition review carrier belongs to another project")
	}
	requestBytes, err := base64.StdEncoding.DecodeString(carrier.RequestCanonicalBase64)
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget,
			fmt.Errorf("decode Transition review request: %w", err)
	}
	request, err := projecttypeenvselection.DecodeProjectTypeEnvHeadSelectionRequest(requestBytes)
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget, err
	}
	if request.Project().String() != carrier.ProjectID ||
		request.Ref().String() != carrier.RequestRef ||
		request.Ref().Digest().String() != carrier.RequestDigest ||
		request.Target().Stage().String() != carrier.StageRef {
		return emptyRequest, emptyContent, emptyStage, emptyTarget,
			fmt.Errorf("transition review request coordinates do not match carrier")
	}
	store, err := projecttypeenvstage.OpenExisting(ctx, ledger.Database())
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget, err
	}
	ready, err := store.LoadSelectionReady(ctx, request.Target().Stage())
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget, err
	}
	stage := ready.Stage()
	predecessor, ok := request.Predecessor().(projecttypeenvselection.TransitionStagePredecessor)
	if !ok {
		return emptyRequest, emptyContent, emptyStage, emptyTarget,
			fmt.Errorf("transition review request has no exact prior head")
	}
	priorHead, err := projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           request.Project(),
			SelectedComposite: predecessor.SelectedComposite(),
			Revision:          predecessor.HeadRevision(),
		},
	)
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget, err
	}
	if err := projecttypeenvselection.VerifyTransitionProjectTypeEnvHeadSelectionRequestStructure(
		request,
		priorHead,
		stage,
	); err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget, err
	}
	contentDigest, err := authority.NewDigest(carrier.AuthorizationContentDigest)
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget, err
	}
	contentCanonical, err := base64.StdEncoding.DecodeString(
		carrier.AuthorizationContentCanonicalBase64,
	)
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget,
			fmt.Errorf("decode Transition authorization content: %w", err)
	}
	content, err := projecttypeenvselectionauthority.DecodeProjectTypeEnvHeadSelectionAuthorizationContent(
		request,
		stage,
		contentCanonical,
		contentDigest,
	)
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget, err
	}
	preparedAt, err := time.Parse(time.RFC3339Nano, carrier.PreparedAt)
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget,
			fmt.Errorf("parse Transition review preparation time: %w", err)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, carrier.ExpiresAt)
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget,
			fmt.Errorf("parse Transition review expiration time: %w", err)
	}
	validity := content.ValidityWindow()
	if !preparedAt.Equal(validity.From()) || !expiresAt.Equal(validity.Until()) {
		return emptyRequest, emptyContent, emptyStage, emptyTarget,
			fmt.Errorf("transition review times differ from authorization content")
	}
	target, err := loadExactReviewedMemoryTarget(ctx, ledger, stage)
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget, err
	}
	if target.Composite().Ref() != request.Target().VerifiedComposite() {
		return emptyRequest, emptyContent, emptyStage, emptyTarget,
			fmt.Errorf("current installed Local-Practice composite differs from reviewed Transition target")
	}
	diff, profiles, err := verifyTransitionReviewDerivations(ctx, ledger, store, stage, carrier)
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget, err
	}
	expected, err := projectTypeEnvTransitionReviewResultFromValues(
		stage,
		request,
		diff,
		profiles,
		validity,
	)
	if err != nil {
		return emptyRequest, emptyContent, emptyStage, emptyTarget, err
	}
	if !sameTransitionReviewNarrative(expected, carrier) {
		return emptyRequest, emptyContent, emptyStage, emptyTarget,
			fmt.Errorf("transition review narrative differs from the exact prepared Stage")
	}
	if carrier.Review.Readiness.Posture != "selectable" {
		return emptyRequest, emptyContent, emptyStage, emptyTarget,
			fmt.Errorf("transition review is not selection-ready: %s", carrier.Review.Readiness.Repair)
	}
	return request, content, stage, target, nil
}

func verifyTransitionReviewDerivations(
	ctx context.Context,
	ledger *projectledger.Handle,
	store *projecttypeenvstage.Store,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	carrier projectTypeEnvTransitionReviewCarrier,
) (
	projecttypeenvcompatibility.SuccessorDiff,
	projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet,
	error,
) {
	predecessor, ok := stage.Predecessor().(projecttypeenvselection.TransitionStagePredecessor)
	if !ok {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{},
			fmt.Errorf("reviewed Stage is not a Transition")
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, ledger.Database())
	if err != nil {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	prior, err := store.LoadExecutableSnapshotTx(ctx, transaction, predecessor.SelectedComposite())
	if err != nil {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{}, err
	}
	target, err := store.LoadExecutableSnapshotTx(ctx, transaction, stage.VerifiedComposite())
	if err != nil {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{}, err
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{}, finish.Err()
	}
	diff, err := projecttypeenvcompatibility.CompareSuccessor(
		prior.Environment(),
		target.Environment(),
	)
	if err != nil {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{}, err
	}
	profiles, err := projecttypeenvprofilecompatibility.AssessTransitionProjectionProfiles(diff)
	if err != nil {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{}, err
	}
	profileBytes, err := base64.StdEncoding.DecodeString(carrier.TransitionReviewArtifactBase64)
	if err != nil {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{}, err
	}
	storedProfiles, err := projecttypeenvprofilecompatibility.DecodeTransitionProjectionProfileCompatibilitySet(profileBytes)
	if err != nil || storedProfiles.Digest().String() != carrier.TransitionReviewArtifactDigest ||
		!bytes.Equal(storedProfiles.CanonicalBytes(), profiles.CanonicalBytes()) {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{},
			fmt.Errorf("transition projection profiles differ from current exact derivation")
	}
	if !bytes.Equal(
		storedProfiles.SuccessorDiff().CanonicalBytes(),
		diff.CanonicalBytes(),
	) {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{},
			fmt.Errorf("transition successor diff differs from current exact derivation")
	}
	stageProfiles, present := stage.TransitionProjectionProfileCompatibility()
	if !present || !bytes.Equal(stageProfiles.CanonicalBytes(), profiles.CanonicalBytes()) {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{},
			fmt.Errorf("transition Stage differs from current exact projection-profile derivation")
	}
	decodedProfiles, err := projecttypeenvprofilecompatibility.DecodeTransitionProjectionProfiles(profiles)
	if err != nil {
		return projecttypeenvcompatibility.SuccessorDiff{},
			projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet{}, err
	}
	return diff, decodedProfiles, nil
}

func projectTypeEnvTransitionReviewResultFromValues(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	diff projecttypeenvcompatibility.SuccessorDiff,
	profiles projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet,
	validity authority.TimeWindow,
) (projectTypeEnvTransitionReviewResponse, error) {
	prior, ok := request.Predecessor().(projecttypeenvselection.TransitionStagePredecessor)
	if !ok {
		return projectTypeEnvTransitionReviewResponse{}, fmt.Errorf("transition predecessor is required")
	}
	projectProfilePosture, err := projectTypeEnvProfilePosture(stage.ProfileCompatibility())
	if err != nil {
		return projectTypeEnvTransitionReviewResponse{}, err
	}
	readiness, nextHumanGate := projectTypeEnvTransitionReadiness(stage, projectProfilePosture, profiles)
	response := projectTypeEnvTransitionReviewResponse{
		ContractVersion: projectTypeEnvTransitionReviewSchema,
		Action:          "prepare",
		Result:          "prepared_successor",
		ProjectID:       stage.Project().String(),
		Review: projectTypeEnvTransitionHumanReview{
			Title:  "Select the reviewed project TypeEnv successor",
			Choice: "Move this project's exact validation, admission, and memory-read ontology from the current composite to the bundled successor",
			WhyNow: "The bundled FPF and Haft Local-Practice target differs from the exact project-selected TypeEnv; shipping it alone does not change project memory semantics",
			Changes: []string{
				"advances ProjectTypeEnvHead by one revision from the exact reviewed predecessor",
				"selects the reviewed successor composite for future validation, admission, and typed reads",
				"records one bounded CAS Work and advances the typed-memory graph by one revision",
				"invalidates successor-sensitive read caches while preserving old snapshot coordinates",
			},
			DoesNotChange: []string{
				"historical assertions or their original TypeEnv basis",
				"project decisions, specifications, source code, or unrelated Work",
				"immutable prior B/E/X/C, Stage, profile, or receipt bytes",
				"release, publication, WorkCommission, or specification authority",
			},
			Validity: projectTypeEnvGenesisReviewValidity{
				From:  validity.From().Format(time.RFC3339Nano),
				Until: validity.Until().Format(time.RFC3339Nano),
			},
			Readiness:       readiness,
			ReturnCondition: "prepare a fresh review if the head, graph, profile catalog, project profile, bundled FPF base, Local-Practice source, or installed runtime changes",
		},
		Candidate: projectTypeEnvTransitionCandidateResponse{
			StageRef:                  stage.Ref().String(),
			PriorHeadRevision:         prior.HeadRevision().Value(),
			PriorCompositeTypeEnvRef:  prior.SelectedComposite().String(),
			BaseTypeEnvRef:            stage.Base().String(),
			ExtensionCount:            len(stage.OrderedExtensions()),
			RuntimeEvaluationBasisRef: stage.RuntimeBasis().String(),
			CompositeTypeEnvRef:       stage.VerifiedComposite().String(),
			GraphRevision:             stage.GraphRevision().Value(),
			Compatibility:             transitionCompatibilitySummary(diff),
			RevalidationPosture:       stage.ExistingAssertionRevalidation().Posture().String(),
			ProjectProfilePosture:     projectProfilePosture,
			ProjectionProfiles:        transitionProfileResponses(profiles),
		},
		Interpretation: projectTypeEnvGenesisReviewInterpretation{
			Establishes: []string{
				"one exact non-binding successor B/E/X/C/Stage candidate persisted in the project ledger",
				"one exact compatibility and revalidation review against the current head, graph, and profiles",
				"one exact review carrier for later manual selection",
			},
			DoesNotEstablish: []string{
				"ProjectTypeEnvHead selection",
				"typed-memory admission authority under the successor",
				"migration or reinterpretation of historical values and assertions",
				"spec lifecycle approval, release readiness, or authority for other Work",
			},
			NextHumanGate: nextHumanGate,
		},
	}
	return response, nil
}

func sameTransitionReviewNarrative(
	expected projectTypeEnvTransitionReviewResponse,
	carrier projectTypeEnvTransitionReviewCarrier,
) bool {
	expectedBytes, err := json.Marshal(struct {
		Review         projectTypeEnvTransitionHumanReview       `json:"review"`
		Candidate      projectTypeEnvTransitionCandidateResponse `json:"candidate"`
		Interpretation projectTypeEnvGenesisReviewInterpretation `json:"interpretation"`
	}{expected.Review, expected.Candidate, expected.Interpretation})
	if err != nil {
		return false
	}
	actualBytes, err := json.Marshal(struct {
		Review         projectTypeEnvTransitionHumanReview       `json:"review"`
		Candidate      projectTypeEnvTransitionCandidateResponse `json:"candidate"`
		Interpretation projectTypeEnvGenesisReviewInterpretation `json:"interpretation"`
	}{carrier.Review, carrier.Candidate, carrier.Interpretation})
	return err == nil && bytes.Equal(expectedBytes, actualBytes)
}

func decodeStrictTransitionReviewJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode Transition review carrier: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("transition review carrier has trailing material")
	}
	return nil
}

func projectTypeEnvTransitionResultResponse(
	projectID string,
	result projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionResult,
) (projectTypeEnvGenesisSelectionResponse, error) {
	response, err := projectTypeEnvGenesisResultResponse(projectID, result)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	response.ContractVersion = "haft.project-typeenv.transition-selection/v1"
	if notSelected, ok := response.Outcome.(projectTypeEnvGenesisNotSelected); ok {
		notSelected.Repair = transitionSelectionRepair(notSelected.Reason)
		response.Outcome = notSelected
	}
	return response, nil
}

func transitionSelectionRepair(reason string) string {
	switch reason {
	case "prior_head_absent":
		return "establish the first project TypeEnv through the separate Genesis prepare/select path"
	case "stale_prior_head":
		return "the project head changed; prepare a fresh successor review against the exact current head"
	case "review_expired":
		return "prepare a fresh exact successor review; another h-decide cannot revive expired authorization content"
	case "profile_incompatible":
		return "inspect both the project declaration profile and installed projection-profile report; repair the blocked basis, then prepare a fresh successor review"
	case "profile_underdetermined", "profile_drift":
		return "repair or review the current project-profile basis, then prepare a fresh successor review"
	case "stale_graph", "stage_drift", "assertion_revalidation_failure":
		return "prepare a fresh successor review against the current graph and assertion basis"
	default:
		return "inspect the exact Transition rejection basis before preparing or selecting another candidate"
	}
}
