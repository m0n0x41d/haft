package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/initialprofilebootstrap"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/onboarding"
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/profileonboarding"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvreviewcarrier"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

type serveProjectOnboardSurface interface {
	Handler() fpf.MemoryToolHandler
	Close() error
}

type serveProjectOnboardSurfaceOpener func(
	context.Context,
	ProjectBinding,
) (serveProjectOnboardSurface, error)

var openServeProjectOnboardSurface serveProjectOnboardSurfaceOpener = func(
	ctx context.Context,
	binding ProjectBinding,
) (serveProjectOnboardSurface, error) {
	return openSealedProjectOnboardSurface(ctx, binding)
}

type sealedProjectOnboardSurface struct {
	service *onboarding.Service
	mu      sync.Mutex
}

func openSealedProjectOnboardSurface(
	ctx context.Context,
	binding ProjectBinding,
) (*sealedProjectOnboardSurface, error) {
	if ctx == nil {
		return nil, fmt.Errorf(
			"open project onboarding surface: context is required",
		)
	}
	if strings.TrimSpace(binding.ProjectRoot) == "" ||
		strings.TrimSpace(binding.ProjectID) == "" {
		return nil, fmt.Errorf(
			"open project onboarding surface: exact project binding is required",
		)
	}
	runtime := &projectOnboardingRuntime{
		binding: binding,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	observation, err := runtime.Observe(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"open project onboarding surface: read startup state: %w",
			err,
		)
	}
	service, err := onboarding.NewService(
		runtime,
		observation.MemoryReady(),
	)
	if err != nil {
		return nil, err
	}
	return &sealedProjectOnboardSurface{
		service: &service,
	}, nil
}

func (surface *sealedProjectOnboardSurface) Handler() fpf.MemoryToolHandler {
	if surface == nil || surface.service == nil {
		return nil
	}
	return func(
		ctx context.Context,
		arguments json.RawMessage,
	) (string, error) {
		surface.mu.Lock()
		defer surface.mu.Unlock()
		if surface.service == nil {
			return "", fmt.Errorf(
				"project onboarding surface is closed",
			)
		}
		return executeOnboardMCP(
			ctx,
			*surface.service,
			arguments,
		)
	}
}

func (surface *sealedProjectOnboardSurface) Close() error {
	if surface == nil {
		return nil
	}
	surface.mu.Lock()
	defer surface.mu.Unlock()
	surface.service = nil
	return nil
}

type fixedOnboardingRuntime struct {
	observation onboarding.Observation
}

func (runtime fixedOnboardingRuntime) Observe(
	context.Context,
) (onboarding.Observation, error) {
	return runtime.observation, nil
}

func (fixedOnboardingRuntime) PrepareProfile(
	context.Context,
	onboarding.Request,
) (onboarding.Preparation, error) {
	return onboarding.Preparation{}, fmt.Errorf(
		"profile preparation is unavailable before Haft initialization",
	)
}

func (fixedOnboardingRuntime) PrepareProfileChange(
	context.Context,
	onboarding.Request,
) (onboarding.Preparation, error) {
	return onboarding.Preparation{}, fmt.Errorf(
		"profile change preparation is unavailable before Haft initialization",
	)
}

func (fixedOnboardingRuntime) PrepareMemory(
	context.Context,
) (onboarding.Preparation, error) {
	return onboarding.Preparation{}, fmt.Errorf(
		"memory preparation is unavailable before Haft initialization",
	)
}

func newOnboardingRequiredMCPHandler() (
	fpf.MemoryToolHandler,
	error,
) {
	observation, err := onboarding.NewObservation(
		onboarding.ObservationInput{
			Initialized: false,
		},
	)
	if err != nil {
		return nil, err
	}
	service, err := onboarding.NewService(
		fixedOnboardingRuntime{
			observation: observation,
		},
		false,
	)
	if err != nil {
		return nil, err
	}
	return func(
		ctx context.Context,
		arguments json.RawMessage,
	) (string, error) {
		return executeOnboardMCP(
			ctx,
			service,
			arguments,
		)
	}, nil
}

type onboardRequestWire struct {
	Action    string              `json:"action"`
	Scopes    *[]onboardScopeWire `json:"scopes,omitempty"`
	Basis     *string             `json:"basis,omitempty"`
	ScopeID   *string             `json:"scope_id,omitempty"`
	EntityRef *string             `json:"entity_ref,omitempty"`
}

type onboardScopeWire struct {
	ScopeID         string   `json:"scope_id"`
	Label           string   `json:"label"`
	RealizationKind string   `json:"realization_kind"`
	EvidencePaths   []string `json:"evidence_paths"`
}

type onboardResponseWire struct {
	Action                     string                     `json:"action"`
	Result                     string                     `json:"result"`
	Status                     string                     `json:"status"`
	Detail                     string                     `json:"detail"`
	NextAction                 string                     `json:"next_action"`
	ReviewRef                  string                     `json:"review_ref,omitempty"`
	ProfileOrigin              string                     `json:"profile_origin,omitempty"`
	AutomaticBootstrapEligible bool                       `json:"automatic_bootstrap_eligible,omitempty"`
	ProfileOverrideEligible    bool                       `json:"profile_override_eligible,omitempty"`
	ProfileChangeEligible      bool                       `json:"profile_change_eligible,omitempty"`
	Scopes                     []onboardScopeResponseWire `json:"scopes,omitempty"`
	Choices                    []string                   `json:"choices,omitempty"`
	Effects                    onboardEffectsWire         `json:"effects"`
	StateDomain                string                     `json:"state_domain"`
	ReadyFor                   []string                   `json:"ready_for"`
	DoesNotEstablish           []string                   `json:"does_not_establish"`
}

type onboardScopeResponseWire struct {
	ScopeID                string   `json:"scope_id"`
	Label                  string   `json:"label"`
	RealizationKind        string   `json:"realization_kind"`
	EvidencePaths          []string `json:"evidence_paths"`
	EvidencePathCount      int      `json:"evidence_path_count"`
	EvidencePathsTruncated bool     `json:"evidence_paths_truncated"`
}

type onboardEffectsWire struct {
	RepositoryInspected     bool `json:"repository_inspected"`
	ReviewCarrierCreated    bool `json:"review_carrier_created"`
	ReviewCarrierReused     bool `json:"review_carrier_reused"`
	CanonicalProfileChanged bool `json:"canonical_profile_changed"`
	StructuredMemoryEnabled bool `json:"structured_memory_enabled"`
	AuthorityGranted        bool `json:"authority_granted"`
}

func executeOnboardMCP(
	ctx context.Context,
	service onboarding.Service,
	arguments json.RawMessage,
) (string, error) {
	request, err := decodeOnboardMCPRequest(arguments)
	if err != nil {
		return "", err
	}
	outcome, err := service.Execute(ctx, request)
	if err != nil {
		outcome = onboarding.BlockedOutcome(
			request.Action(),
			fallbackOnboardingStatus(request.Action()),
			"Haft could not read or prepare the requested setup state safely; no binding setup change was claimed.",
			"Inspect the current project setup and retry the same readable onboarding action.",
		)
	}
	response := presentOnboardOutcome(outcome)
	encoded, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf(
			"encode onboarding response: %w",
			err,
		)
	}
	return string(encoded), nil
}

func decodeOnboardMCPRequest(
	arguments json.RawMessage,
) (onboarding.Request, error) {
	if err := typedmemorywire.ValidateStrictJSON(arguments); err != nil {
		return onboarding.Request{}, err
	}
	wire := onboardRequestWire{}
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return onboarding.Request{}, fmt.Errorf(
			"decode haft_onboard request: %w",
			err,
		)
	}
	if err := requireOnboardRequestEOF(decoder); err != nil {
		return onboarding.Request{}, err
	}
	scopes, err := onboardScopesFromWire(wire.Scopes)
	if err != nil {
		return onboarding.Request{}, err
	}
	input := onboarding.RequestInput{
		Action:           wire.Action,
		BasisPresent:     wire.Basis != nil,
		ScopesPresent:    wire.Scopes != nil,
		Scopes:           scopes,
		ScopeIDPresent:   wire.ScopeID != nil,
		EntityRefPresent: wire.EntityRef != nil,
	}
	if wire.Basis != nil {
		input.Basis = *wire.Basis
	}
	if wire.ScopeID != nil {
		input.ScopeID = *wire.ScopeID
	}
	if wire.EntityRef != nil {
		input.EntityRef = *wire.EntityRef
	}
	return onboarding.NewRequest(input)
}

func requireOnboardRequestEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf(
			"decode trailing haft_onboard request: %w",
			err,
		)
	}
	return fmt.Errorf(
		"haft_onboard request contains multiple JSON values",
	)
}

func onboardScopesFromWire(
	values *[]onboardScopeWire,
) ([]onboarding.Scope, error) {
	if values == nil {
		return nil, nil
	}
	scopes := make([]onboarding.Scope, len(*values))
	for index, value := range *values {
		kind := onboarding.RealizationKind(
			value.RealizationKind,
		)
		scope, err := onboarding.NewScope(
			value.ScopeID,
			value.Label,
			kind,
			value.EvidencePaths,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"profile_prepare scope %d: %w",
				index,
				err,
			)
		}
		scopes[index] = scope
	}
	return scopes, nil
}

func presentOnboardOutcome(
	outcome onboarding.Outcome,
) onboardResponseWire {
	effects := outcome.Effects()
	return onboardResponseWire{
		Action:                     string(outcome.Action()),
		Result:                     string(outcome.Result()),
		Status:                     string(outcome.Status()),
		Detail:                     outcome.Detail(),
		NextAction:                 outcome.NextAction(),
		ReviewRef:                  outcome.ReviewRef(),
		ProfileOrigin:              string(outcome.ProfileOrigin()),
		AutomaticBootstrapEligible: outcome.AutoBootstrapEligible(),
		ProfileOverrideEligible:    outcome.ProfileOverrideEligible(),
		ProfileChangeEligible:      outcome.ProfileOrigin() != "",
		Scopes:                     onboardScopesToWire(outcome.Scopes()),
		Choices:                    outcome.Choices(),
		StateDomain:                outcome.StateDomain(),
		ReadyFor:                   outcome.ReadyFor(),
		DoesNotEstablish:           outcome.DoesNotEstablish(),
		Effects: onboardEffectsWire{
			RepositoryInspected:     effects.RepositoryInspected,
			ReviewCarrierCreated:    effects.ReviewCarrierCreated,
			ReviewCarrierReused:     effects.ReviewCarrierReused,
			CanonicalProfileChanged: effects.CanonicalProfileChanged,
			StructuredMemoryEnabled: effects.StructuredMemoryEnabled,
			AuthorityGranted:        effects.AuthorityGranted,
		},
	}
}

func onboardScopesToWire(
	values []onboarding.Scope,
) []onboardScopeResponseWire {
	result := make([]onboardScopeResponseWire, len(values))
	for index, value := range values {
		result[index] = onboardScopeResponseWire{
			ScopeID:                value.ScopeID(),
			Label:                  value.Label(),
			RealizationKind:        string(value.RealizationKind()),
			EvidencePaths:          value.EvidencePaths(),
			EvidencePathCount:      value.EvidencePathCount(),
			EvidencePathsTruncated: value.EvidencePathsTruncated(),
		}
	}
	return result
}

func fallbackOnboardingStatus(
	action onboarding.Action,
) onboarding.Status {
	if action == onboarding.ActionMemoryPrepare {
		return onboarding.StatusNeedsMemory
	}
	return onboarding.StatusNeedsProfile
}

type projectOnboardingRuntime struct {
	binding ProjectBinding
	now     func() time.Time
}

func (runtime *projectOnboardingRuntime) Observe(
	ctx context.Context,
) (onboarding.Observation, error) {
	inspection, suggestion, err :=
		executeProfileInspectionWithSuggestion(
			ctx,
			runtime.binding.ProjectRoot,
			false,
		)
	if err != nil {
		return onboarding.Observation{}, err
	}
	if inspection.CanonicalProfile.Kind != "declared" {
		return runtime.observePendingProfile(
			suggestion,
			inspection,
		)
	}
	scopes, err := onboardScopesFromCanonicalProfile(
		inspection.CanonicalProfile.Scopes,
	)
	if err != nil {
		return onboarding.Observation{}, err
	}
	profileChangeReviewReady, profileChangeDetail :=
		inspectCurrentProfileChangeReview(
			runtime.binding.ProjectRoot,
			suggestion,
			inspection.CanonicalProfile,
		)
	memoryReady, err := projectMemoryReadyReadOnly(
		ctx,
		runtime.binding,
	)
	if err != nil {
		return onboarding.Observation{}, err
	}
	reviewReady := false
	memoryDeferred := false
	detail := profileChangeDetail
	if !memoryReady {
		memoryDetail := ""
		memoryDeferred, memoryDetail =
			inspectOnboardMemoryDeferral(runtime)
		if !memoryDeferred && memoryDetail == "" {
			reviewReady, memoryDetail = runtime.inspectMemoryReview()
		}
		detail = strings.TrimSpace(
			strings.Join(
				[]string{profileChangeDetail, memoryDetail},
				" ",
			),
		)
	}
	return onboarding.NewObservation(
		onboarding.ObservationInput{
			Initialized:     true,
			ProfileDeclared: true,
			ProfileOverrideEligible: inspection.CanonicalProfile.Origin ==
				string(projectprofile.ProfileAdmissionOriginDetectorDefault),
			ProfileOrigin: projectprofile.ProfileAdmissionOrigin(
				inspection.CanonicalProfile.Origin,
			),
			MemoryReady:              memoryReady,
			ProfileChangeReviewReady: profileChangeReviewReady,
			MemoryReviewReady:        reviewReady,
			MemoryDeferred:           memoryDeferred,
			Scopes:                   scopes,
			Detail:                   detail,
		},
	)
}

func inspectCurrentProfileChangeReview(
	projectRoot string,
	suggestion profiledetector.Suggestion,
	current canonicalProfileView,
) (bool, string) {
	content, present, err := readOptionalRegularProfileReview(
		profileChangeReviewPath(projectRoot),
	)
	if err != nil {
		return false, "The profile-change review could not be read safely."
	}
	if !present {
		return false, ""
	}
	input, err := profileonboarding.DecodeProfileOnboardingWorkInput(
		content,
		suggestion,
	)
	if err != nil {
		return false, "The existing profile-change review is stale against the current repository observation."
	}
	basis, ok := input.ProfileChangeBasis()
	if !ok {
		return false, "The existing profile-change review does not carry a valid predecessor basis."
	}
	currentMatches := basis.AdmissionRecordRef().String() == current.AdmissionRecordRef &&
		basis.AdmissionRecordDigest().String() == current.AdmissionRecordDigest &&
		basis.PayloadDigest().String() == current.PayloadDigest &&
		basis.LedgerRevision().Value() == current.LedgerRevision
	if !currentMatches {
		return false, "The existing profile-change review is stale against the current canonical profile."
	}
	detail := fmt.Sprintf(
		"A predecessor-pinned profile-change review is ready for scope %q: entity_ref %q -> %q.",
		basis.ScopeID().String(),
		basis.PreviousEntityRef(),
		basis.NextEntityRef().String(),
	)
	return true, detail
}

func (runtime *projectOnboardingRuntime) observePendingProfile(
	suggestion profiledetector.Suggestion,
	inspection profileInspectionResponse,
) (onboarding.Observation, error) {
	detected, err := onboardScopesFromSuggestion(suggestion)
	if err != nil {
		return onboarding.Observation{}, err
	}
	reviewReady, reviewed, reviewDetail :=
		inspectCurrentProfileReview(
			runtime.binding.ProjectRoot,
			suggestion,
			detected,
		)
	scopes := detected
	if reviewReady {
		scopes = reviewed
	}
	detail := reviewDetail
	if detail == "" {
		detail = readableProfileDetectionDetail(
			inspection.Suggestion.Classification,
		)
	}
	autoBootstrapEligible, err := publicOnboardingAutomaticBootstrapEligible(
		runtime.binding.ProjectRoot,
		suggestion,
	)
	if err != nil {
		return onboarding.Observation{}, err
	}
	return onboarding.NewObservation(
		onboarding.ObservationInput{
			Initialized:        true,
			ProfileReviewReady: reviewReady,
			DetectionNeedsHelp: suggestion.ScopeIdentityPosture() !=
				profiledetector.StableScopeIdentity,
			AutoBootstrapEligible: autoBootstrapEligible,
			Scopes:                scopes,
			Detail:                detail,
		},
	)
}

func publicOnboardingAutomaticBootstrapEligible(
	projectRoot string,
	suggestion profiledetector.Suggestion,
) (bool, error) {
	reviewBytes, reviewPresent, err := readOptionalRegularProfileReview(
		profileDeclarationReviewPath(projectRoot),
	)
	if err != nil {
		return false, err
	}
	review := initialprofilebootstrap.ReviewAbsent
	if reviewPresent {
		_, generated := profiledeclarationpreparation.
			InspectGeneratedProfileReview(reviewBytes)
		review = initialprofilebootstrap.ReviewHumanOrForeign
		if generated {
			review = initialprofilebootstrap.ReviewGeneratedUnedited
		}
	}
	decision, err := initialprofilebootstrap.Decide(false, review, suggestion)
	if err != nil {
		return false, err
	}
	return decision.Kind() == initialprofilebootstrap.ApplySupportedSingleton, nil
}

func (runtime *projectOnboardingRuntime) PrepareProfile(
	ctx context.Context,
	request onboarding.Request,
) (onboarding.Preparation, error) {
	inspection, suggestion, err :=
		executeProfileInspectionWithSuggestion(
			ctx,
			runtime.binding.ProjectRoot,
			false,
		)
	if err != nil {
		return onboarding.Preparation{}, err
	}
	declaredProfile := inspection.CanonicalProfile.Kind == "declared"
	detectorDefault := inspection.CanonicalProfile.Origin ==
		string(projectprofile.ProfileAdmissionOriginDetectorDefault)
	if declaredProfile && !detectorDefault {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			nil,
			"The canonical project profile is not detector_default; explicit-over-explicit and legacy profile changes require their own mutation contract.",
		)
	}
	detected, err := onboardScopesFromSuggestion(suggestion)
	if err != nil {
		return onboarding.Preparation{}, err
	}
	if request.HasExplicitScopes() {
		return runtime.prepareManualProfileReview(
			suggestion,
			request,
		)
	}
	if suggestion.ScopeIdentityPosture() !=
		profiledetector.StableScopeIdentity {
		return onboarding.NewPreparation(
			onboarding.PreparationNeedsScopeReview,
			"",
			detected,
			"Repository detection cannot establish stable project scope identity; no review carrier was written.",
		)
	}
	review, err := prepareProfileReviewCandidate(
		runtime.binding.ProjectRoot,
		suggestion,
	)
	if err != nil {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			detected,
			"A different or stale profile review is already present; it was retained unchanged.",
		)
	}
	return profilePreparationFromInstalledReview(
		review.State,
		detected,
	)
}

func (runtime *projectOnboardingRuntime) PrepareProfileChange(
	ctx context.Context,
	request onboarding.Request,
) (preparation onboarding.Preparation, runErr error) {
	inspection, suggestion, err := executeProfileInspectionWithSuggestion(
		ctx,
		runtime.binding.ProjectRoot,
		false,
	)
	if err != nil {
		return onboarding.Preparation{}, err
	}
	if inspection.CanonicalProfile.Kind != "declared" {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			nil,
			"A canonical project profile is required before a scope relation can change.",
		)
	}
	handle, err := projectledger.OpenExisting(
		ctx,
		runtime.binding.ProjectRoot,
		projectledger.ReadOnly,
	)
	if err != nil {
		return onboarding.Preparation{}, err
	}
	defer func() {
		runErr = errors.Join(runErr, handle.Close())
	}()
	service, err := profileadmissionsqlite.NewService(handle.Database())
	if err != nil {
		return onboarding.Preparation{}, err
	}
	root, err := projectprofile.NewProjectRootV1(
		handle.ProjectRoot().String(),
	)
	if err != nil {
		return onboarding.Preparation{}, err
	}
	resolved := service.ResolveCurrent(ctx, root)
	admission, ok := resolved.Admission()
	if !ok {
		return onboarding.Preparation{}, canonicalProfileResolutionError(resolved)
	}
	scopeID, err := projectprofile.NewScopeID(request.ScopeID())
	if err != nil {
		return onboarding.Preparation{}, err
	}
	entityRef, err := projectprofile.NewEntityRef(request.EntityRef())
	if err != nil {
		return onboarding.Preparation{}, err
	}
	previousEntityRef, present := profileScopeEntityRef(
		admission.Payload(),
		scopeID,
	)
	if !present {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			onboardScopesFromCanonicalProfileOrEmpty(
				inspection.CanonicalProfile.Scopes,
			),
			fmt.Sprintf(
				"The canonical profile has no scope_id %q; no change review was written.",
				scopeID.String(),
			),
		)
	}
	basis, err := profileonboarding.NewProfileChangeBasis(
		admission.AdmissionRecordRef(),
		admission.AdmissionRecordDigest(),
		admission.PayloadDigest(),
		admission.LedgerRevision(),
		scopeID,
		previousEntityRef,
		entityRef,
	)
	if err != nil {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			onboardScopesFromCanonicalProfileOrEmpty(
				inspection.CanonicalProfile.Scopes,
			),
			"The selected scope already has that entity_ref; no change review was written.",
		)
	}
	content, err := profileonboarding.
		ProposeProfileEntityRelationChangeWorkInput(
			suggestion,
			admission.Payload(),
			basis,
		)
	if err != nil {
		return onboarding.Preparation{}, err
	}
	if _, err := profileonboarding.DecodeProfileOnboardingWorkInput(
		content,
		suggestion,
	); err != nil {
		return onboarding.Preparation{}, err
	}
	state, err := installProfileChangeReviewCandidate(
		runtime.binding.ProjectRoot,
		content,
	)
	if err != nil {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			onboardScopesFromCanonicalProfileOrEmpty(
				inspection.CanonicalProfile.Scopes,
			),
			"A different or stale profile-change review is already present; it was retained unchanged.",
		)
	}
	kind := onboarding.PreparationCreated
	detail := fmt.Sprintf(
		"A non-binding profile-change review was prepared for scope %q: entity_ref %q -> %q. The canonical profile did not change.",
		scopeID.String(),
		previousEntityRef,
		entityRef.String(),
	)
	if state == "reused" {
		kind = onboarding.PreparationReused
		detail = fmt.Sprintf(
			"The exact non-binding profile-change review was reused for scope %q: entity_ref %q -> %q. The canonical profile did not change.",
			scopeID.String(),
			previousEntityRef,
			entityRef.String(),
		)
	}
	return onboarding.NewPreparation(
		kind,
		"review:onboard-profile-change",
		onboardScopesFromCanonicalProfileOrEmpty(
			inspection.CanonicalProfile.Scopes,
		),
		detail,
	)
}

func profileScopeEntityRef(
	payload projectprofile.ProfileDeclarationPayload,
	scopeID projectprofile.ScopeID,
) (string, bool) {
	for _, scope := range payload.Scopes().Values() {
		if scope.ScopeID() != scopeID {
			continue
		}
		switch value := scope.(type) {
		case projectprofile.SoftwareRealization:
			return entityReferenceText(value.EntityReference()), true
		case projectprofile.NonSoftwareRealization:
			return entityReferenceText(value.EntityReference()), true
		default:
			return "", false
		}
	}
	return "", false
}

func onboardScopesFromCanonicalProfileOrEmpty(
	values []canonicalProfileScopeView,
) []onboarding.Scope {
	scopes, err := onboardScopesFromCanonicalProfile(values)
	if err != nil {
		return nil
	}
	return scopes
}

func (runtime *projectOnboardingRuntime) prepareManualProfileReview(
	suggestion profiledetector.Suggestion,
	request onboarding.Request,
) (onboarding.Preparation, error) {
	if suggestion.ScopeIdentityPosture() !=
		profiledetector.ScopeIdentityNeedsReview {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			request.Scopes(),
			"Explicit fallback scopes are available only when a complete repository observation leaves scope identity underdetermined; the current detected scopes were retained.",
		)
	}
	proposalScopes := make(
		[]profileonboarding.ManualProfileScopeInput,
		len(request.Scopes()),
	)
	for index, scope := range request.Scopes() {
		proposalScopes[index] =
			profileonboarding.ManualProfileScopeInput{
				ScopeID: scope.ScopeID(),
				Label:   scope.Label(),
				RealizationKind: profileRealizationKind(
					scope.RealizationKind(),
				),
				EvidencePaths: scope.EvidencePaths(),
			}
	}
	content, err :=
		profileonboarding.ProposeManualProfileOnboardingWorkInput(
			suggestion,
			profileonboarding.ManualProfileProposalInput{
				Basis:  request.Basis(),
				Scopes: proposalScopes,
			},
		)
	if err != nil {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			request.Scopes(),
			"The explicit scopes do not match the current complete repository observation; no review carrier was written.",
		)
	}
	if _, err := profileonboarding.DecodeProfileOnboardingWorkInput(
		content,
		suggestion,
	); err != nil {
		return onboarding.Preparation{}, err
	}
	state, err := installProfileReviewCandidate(
		runtime.binding.ProjectRoot,
		content,
	)
	if err != nil {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			request.Scopes(),
			"A different or stale profile review is already present; it was retained unchanged.",
		)
	}
	return profilePreparationFromInstalledReview(
		state,
		request.Scopes(),
	)
}

func profilePreparationFromInstalledReview(
	state string,
	scopes []onboarding.Scope,
) (onboarding.Preparation, error) {
	kind := onboarding.PreparationCreated
	detail := "A non-binding project-profile review was prepared. No canonical profile or authority state changed."
	if state == "reused" {
		kind = onboarding.PreparationReused
		detail = "The exact non-binding project-profile review was reused. No canonical profile or authority state changed."
	}
	return onboarding.NewPreparation(
		kind,
		"review:onboard-profile",
		scopes,
		detail,
	)
}

func (runtime *projectOnboardingRuntime) PrepareMemory(
	ctx context.Context,
) (preparation onboarding.Preparation, runErr error) {
	inspection, err := executeProfileInspection(
		ctx,
		runtime.binding.ProjectRoot,
		false,
	)
	if err != nil {
		return onboarding.Preparation{}, err
	}
	if inspection.CanonicalProfile.Kind != "declared" {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			nil,
			"A canonical project profile is required before the structured-memory review can be prepared.",
		)
	}
	scopes, err := onboardScopesFromCanonicalProfile(
		inspection.CanonicalProfile.Scopes,
	)
	if err != nil {
		return onboarding.Preparation{}, err
	}
	ready, err := projectMemoryReadyReadOnly(
		ctx,
		runtime.binding,
	)
	if err != nil {
		return onboarding.Preparation{}, err
	}
	if ready {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			scopes,
			"Structured project memory is already enabled; no review carrier was changed.",
		)
	}
	if reviewReady, _ := runtime.inspectMemoryReview(); reviewReady {
		if err := clearOnboardMemoryDeferral(
			runtime.binding.ProjectRoot,
			runtime.binding.ProjectID,
		); err != nil {
			return onboarding.NewPreparation(
				onboarding.PreparationBlocked,
				"",
				scopes,
				"The current structured-memory choice could not be reopened safely; the existing state was retained.",
			)
		}
		return onboarding.NewPreparation(
			onboarding.PreparationReused,
			"review:onboard-memory",
			scopes,
			"The exact non-binding structured-memory enable/defer review was reused. Structured memory remains disabled until an explicit human choice.",
		)
	}
	ledger, err := projectledger.OpenExisting(
		ctx,
		runtime.binding.ProjectRoot,
		projectledger.ReadWrite,
	)
	if err != nil {
		return onboarding.Preparation{}, err
	}
	defer func() {
		runErr = errors.Join(runErr, ledger.Close())
	}()
	if ledger.ProjectID().String() != runtime.binding.ProjectID {
		return onboarding.Preparation{}, fmt.Errorf(
			"checked project identity changed before memory review preparation",
		)
	}
	embedded, err := loadEmbeddedMemoryRuntime(ctx)
	if err != nil {
		return onboarding.Preparation{}, err
	}
	previousDigest, previousPresent :=
		readCurrentMemoryReviewDigest(
			runtime.binding.ProjectRoot,
		)
	prepared, err := prepareProjectTypeEnvGenesisReview(
		ctx,
		ledger,
		embedded.Artifact(),
		typedmemorystore.SystemClock{},
	)
	if err != nil {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			scopes,
			"The structured-memory review could not be prepared safely; no enablement or authority change was claimed.",
		)
	}
	if prepared.response.Review.Readiness.Posture != "selectable" {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			scopes,
			"The structured-memory candidate is not ready for the enable/defer review; no enablement or authority change occurred.",
		)
	}
	installReview := writeProjectTypeEnvGenesisReview
	if previousPresent {
		installReview = replaceProjectTypeEnvGenesisReview
	}
	digest, err := installReview(
		runtime.binding.ProjectRoot,
		prepared.carrier,
	)
	if err != nil {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			scopes,
			"A different structured-memory review is already present; it was retained unchanged.",
		)
	}
	if err := ledger.Revalidate(ctx); err != nil {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			scopes,
			"The review may exist, but the current project attachment could not be rechecked; inspect it before any human choice.",
		)
	}
	if err := clearOnboardMemoryDeferral(
		runtime.binding.ProjectRoot,
		runtime.binding.ProjectID,
	); err != nil {
		return onboarding.NewPreparation(
			onboarding.PreparationBlocked,
			"",
			scopes,
			"The review may exist, but the current structured-memory choice could not be reopened safely; inspect onboarding status before making a new choice.",
		)
	}
	kind := onboarding.PreparationCreated
	detail := "A non-binding structured-memory enable/defer review was prepared. Structured memory remains disabled until an explicit human choice."
	if previousPresent && previousDigest == digest {
		kind = onboarding.PreparationReused
		detail = "The exact non-binding structured-memory enable/defer review was reused. Structured memory remains disabled until an explicit human choice."
	}
	return onboarding.NewPreparation(
		kind,
		"review:onboard-memory",
		scopes,
		detail,
	)
}

func profileRealizationKind(
	value onboarding.RealizationKind,
) profiledetector.RealizationKind {
	if value == onboarding.Software {
		return profiledetector.SoftwareRealization
	}
	return profiledetector.NonSoftwareRealization
}

func onboardScopesFromSuggestion(
	suggestion profiledetector.Suggestion,
) ([]onboarding.Scope, error) {
	if suggestion.ScopeIdentityPosture() !=
		profiledetector.StableScopeIdentity {
		return []onboarding.Scope{}, nil
	}
	values := suggestion.SuggestedScopes()
	result := make([]onboarding.Scope, len(values))
	for index, value := range values {
		paths := make(
			[]string,
			len(value.PositiveSignals()),
		)
		for pathIndex, signal := range value.PositiveSignals() {
			paths[pathIndex] = signal.Path()
		}
		slices.Sort(paths)
		paths = slices.Compact(paths)
		evidencePathCount := len(paths)
		if len(paths) > onboarding.MaximumEvidencePaths {
			paths = append(
				[]string{},
				paths[:onboarding.MaximumEvidencePaths]...,
			)
		}
		scope, err := onboarding.NewProjectedScope(
			value.Orientation(),
			readableScopeLabel(value.Orientation()),
			onboarding.RealizationKind(value.RealizationKind()),
			paths,
			evidencePathCount,
		)
		if err != nil {
			return nil, err
		}
		result[index] = scope
	}
	return result, nil
}

func onboardScopesFromCanonicalProfile(
	values []canonicalProfileScopeView,
) ([]onboarding.Scope, error) {
	result := make([]onboarding.Scope, len(values))
	for index, value := range values {
		scope, err := onboarding.NewScope(
			value.ScopeID,
			readableScopeLabel(value.ScopeID),
			onboarding.RealizationKind(value.RealizationKind),
			nil,
		)
		if err != nil {
			return nil, err
		}
		result[index] = scope
	}
	return result, nil
}

func readableScopeLabel(value string) string {
	words := strings.FieldsFunc(
		value,
		func(character rune) bool {
			return character == '-' ||
				character == '_' ||
				character == '/'
		},
	)
	if len(words) == 0 {
		return value
	}
	for index, word := range words {
		if word == "" {
			continue
		}
		words[index] = strings.ToUpper(word[:1]) +
			word[1:]
	}
	return strings.Join(words, " ")
}

type readableProfileReviewCarrier struct {
	ManualBasis string                       `json:"manual_basis"`
	Scopes      []readableProfileReviewScope `json:"scopes"`
}

type readableProfileReviewScope struct {
	ScopeID         string   `json:"scope_id"`
	Label           string   `json:"label"`
	RealizationKind string   `json:"realization_kind"`
	EvidencePaths   []string `json:"evidence_paths"`
}

func inspectCurrentProfileReview(
	projectRoot string,
	suggestion profiledetector.Suggestion,
	detected []onboarding.Scope,
) (bool, []onboarding.Scope, string) {
	content, present, err := readOptionalRegularProfileReview(
		profileDeclarationReviewPath(projectRoot),
	)
	if err != nil {
		return false, detected,
			"The existing profile review is unreadable; it was not treated as current."
	}
	if !present {
		return false, detected, ""
	}
	input, err := profileonboarding.DecodeProfileOnboardingWorkInput(
		content,
		suggestion,
	)
	if err != nil {
		return false, detected,
			"The existing profile review does not match the current repository observation; it was retained but not treated as current."
	}
	reviewed, err := readableScopesFromProfileInput(
		input,
		detected,
	)
	if err != nil {
		return false, detected,
			"The existing profile review could not be projected into readable scopes; it was not treated as current."
	}
	detail := "A non-binding project-profile review matches the current repository observation."
	if input.UsesManualScopeBasis() {
		detail = "A non-binding manual project-profile review matches the current repository observation. Re-prepare it if scope, label, kind, evidence, or basis changes."
	}
	return true, reviewed, detail
}

func readableScopesFromProfileInput(
	input profileonboarding.ProfileOnboardingWorkInput,
	detected []onboarding.Scope,
) ([]onboarding.Scope, error) {
	wire := readableProfileReviewCarrier{}
	if err := json.Unmarshal(input.CanonicalJSON(), &wire); err != nil {
		return nil, err
	}
	detectedByID := make(
		map[string]onboarding.Scope,
		len(detected),
	)
	for _, scope := range detected {
		detectedByID[scope.ScopeID()] = scope
	}
	result := make([]onboarding.Scope, len(wire.Scopes))
	for index, value := range wire.Scopes {
		label := value.Label
		if label == "" {
			label = readableScopeLabel(value.ScopeID)
		}
		evidence := value.EvidencePaths
		evidencePathCount := len(evidence)
		if len(evidence) == 0 {
			if detectedScope, present :=
				detectedByID[value.ScopeID]; present {
				evidence = detectedScope.EvidencePaths()
				evidencePathCount = detectedScope.EvidencePathCount()
			}
		}
		scope, err := onboarding.NewProjectedScope(
			value.ScopeID,
			label,
			onboarding.RealizationKind(
				value.RealizationKind,
			),
			evidence,
			evidencePathCount,
		)
		if err != nil {
			return nil, err
		}
		result[index] = scope
	}
	return result, nil
}

func readableProfileDetectionDetail(
	classification string,
) string {
	switch profiledetector.Classification(classification) {
	case profiledetector.SoftwareSignals:
		return "Repository detection found a software project scope; the result remains advisory until reviewed and explicitly applied."
	case profiledetector.NonSoftwareSignals:
		return "Repository detection found one or more non-software project scopes; the result remains advisory until reviewed and explicitly applied."
	case profiledetector.MixedSignals:
		return "Repository detection found multiple software and non-software scopes; the result remains advisory until reviewed and explicitly applied."
	default:
		return "Repository detection cannot establish a reliable scope; an explicit readable fallback is required."
	}
}

func (runtime *projectOnboardingRuntime) inspectMemoryReview() (
	bool,
	string,
) {
	path := projectTypeEnvGenesisReviewPath(
		runtime.binding.ProjectRoot,
	)
	_, statErr := os.Lstat(path)
	if errors.Is(statErr, os.ErrNotExist) {
		return false, ""
	}
	if statErr != nil {
		return false,
			"The existing structured-memory review is unreadable; it was not treated as current."
	}
	carrier, err := readProjectTypeEnvGenesisReview(
		runtime.binding.ProjectRoot,
	)
	if err != nil {
		return false,
			"The existing structured-memory review is invalid; it was not treated as current."
	}
	if carrier.ProjectID != runtime.binding.ProjectID {
		return false,
			"The existing structured-memory review belongs to a different project; it was not treated as current."
	}
	if carrier.Review.Readiness.Posture != "selectable" {
		return false,
			"The existing structured-memory review is not selectable; prepare a fresh review after resolving its readable blockers."
	}
	expiresAt, err := time.Parse(
		time.RFC3339Nano,
		carrier.ExpiresAt,
	)
	if err != nil || runtime.now == nil ||
		runtime.now().After(expiresAt) {
		return false,
			"The existing structured-memory review is stale; prepare a fresh review before presenting a choice."
	}
	return true,
		"A non-binding structured-memory enable/defer review is ready."
}

func readCurrentMemoryReviewDigest(
	projectRoot string,
) (string, bool) {
	path := projectTypeEnvGenesisReviewPath(projectRoot)
	if _, err := os.Lstat(path); err != nil {
		return "", false
	}
	carrier, err := projecttypeenvreviewcarrier.Read(projectRoot)
	if err != nil {
		return "", false
	}
	return carrier.Digest().String(), true
}

func projectMemoryReadyReadOnly(
	ctx context.Context,
	binding ProjectBinding,
) (bool, error) {
	basis, err := observePublicTypeEnvBasis(
		ctx,
		binding.ProjectRoot,
		binding.ProjectID,
	)
	if err != nil {
		return false, err
	}
	switch basis.Kind() {
	case initplanning.BasisUnavailable:
		return false, nil
	case initplanning.BasisSelected:
		return true, nil
	default:
		return false, fmt.Errorf(
			"current structured-memory readiness has an unsupported state",
		)
	}
}
