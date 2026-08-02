// Package onboarding defines the task-level, non-binding project setup
// contract. It deliberately knows nothing about CLI flags, MCP JSON, project
// schema coordinates, or persistence implementation details.
package onboarding

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type Action string

const (
	ActionStatus         Action = "status"
	ActionProfilePrepare Action = "profile_prepare"
	ActionMemoryPrepare  Action = "memory_prepare"
)

type Status string

const (
	StatusNeedsInit          Status = "needs_init"
	StatusNeedsProfile       Status = "needs_profile"
	StatusProfileReviewReady Status = "profile_review_ready"
	StatusNeedsMemory        Status = "needs_memory"
	StatusMemoryReviewReady  Status = "memory_review_ready"
	StatusMemoryDeferred     Status = "memory_deferred"
	StatusReady              Status = "ready"
)

type Result string

const (
	ResultOnboardingRequired   Result = "onboarding_required"
	ResultNeedsProfile         Result = "needs_profile"
	ResultNeedsScopeReview     Result = "needs_scope_review"
	ResultProfileReviewReady   Result = "profile_review_ready"
	ResultProfileReviewCreated Result = "profile_review_prepared"
	ResultProfileReviewReused  Result = "profile_review_reused"
	ResultNeedsMemory          Result = "needs_memory"
	ResultMemoryReviewReady    Result = "memory_review_ready"
	ResultMemoryReviewCreated  Result = "memory_review_prepared"
	ResultMemoryReviewReused   Result = "memory_review_reused"
	ResultMemoryDeferred       Result = "memory_deferred"
	ResultRestartRequired      Result = "restart_required"
	ResultReady                Result = "ready"
	ResultBlocked              Result = "blocked"
)

type RealizationKind string

const (
	Software    RealizationKind = "software"
	NonSoftware RealizationKind = "non_software"
)

const (
	EnableStructuredMemoryChoice = "Enable structured project memory"
	DeferStructuredMemoryChoice  = "Not now"
)

const (
	MaximumProfileScopes     = 32
	MaximumScopeIDBytes      = 200
	MaximumScopeLabelBytes   = 200
	MaximumProfileBasisBytes = 4096
	MaximumEvidencePaths     = 64
	MaximumEvidencePathBytes = 4096
)

type Scope struct {
	scopeID         string
	label           string
	realizationKind RealizationKind
	evidencePaths   []string
}

func NewScope(
	scopeID string,
	label string,
	realizationKind RealizationKind,
	evidencePaths []string,
) (Scope, error) {
	if err := requireExactReadableText(
		"scope_id",
		scopeID,
		MaximumScopeIDBytes,
	); err != nil {
		return Scope{}, err
	}
	if err := requireExactReadableText(
		"label",
		label,
		MaximumScopeLabelBytes,
	); err != nil {
		return Scope{}, err
	}
	if realizationKind != Software && realizationKind != NonSoftware {
		return Scope{}, fmt.Errorf(
			"realization_kind must be %q or %q",
			Software,
			NonSoftware,
		)
	}
	if len(evidencePaths) > MaximumEvidencePaths {
		return Scope{}, fmt.Errorf(
			"evidence_paths must contain at most %d entries",
			MaximumEvidencePaths,
		)
	}
	paths := append([]string{}, evidencePaths...)
	for _, path := range paths {
		if err := requireExactReadableText(
			"evidence_path",
			path,
			MaximumEvidencePathBytes,
		); err != nil {
			return Scope{}, err
		}
	}
	slices.Sort(paths)
	if len(slices.Compact(append([]string{}, paths...))) != len(paths) {
		return Scope{}, fmt.Errorf("evidence_paths must not repeat")
	}
	return Scope{
		scopeID:         scopeID,
		label:           label,
		realizationKind: realizationKind,
		evidencePaths:   paths,
	}, nil
}

func (scope Scope) ScopeID() string {
	return scope.scopeID
}

func (scope Scope) Label() string {
	return scope.label
}

func (scope Scope) RealizationKind() RealizationKind {
	return scope.realizationKind
}

func (scope Scope) EvidencePaths() []string {
	return append([]string{}, scope.evidencePaths...)
}

type Request struct {
	action Action
	basis  string
	scopes []Scope
}

type RequestInput struct {
	Action        string
	BasisPresent  bool
	Basis         string
	ScopesPresent bool
	Scopes        []Scope
}

func NewRequest(input RequestInput) (Request, error) {
	action := Action(input.Action)
	if action != ActionStatus &&
		action != ActionProfilePrepare &&
		action != ActionMemoryPrepare {
		return Request{}, fmt.Errorf(
			"action must be status, profile_prepare, or memory_prepare",
		)
	}
	if action != ActionProfilePrepare &&
		(input.BasisPresent || input.ScopesPresent) {
		return Request{}, fmt.Errorf(
			"%s accepts only action",
			action,
		)
	}
	if action != ActionProfilePrepare {
		return Request{action: action}, nil
	}
	if input.BasisPresent != input.ScopesPresent {
		return Request{}, fmt.Errorf(
			"profile_prepare requires basis and scopes together for an explicit fallback",
		)
	}
	if !input.BasisPresent {
		return Request{action: action}, nil
	}
	if err := requireExactReadableText(
		"basis",
		input.Basis,
		MaximumProfileBasisBytes,
	); err != nil {
		return Request{}, err
	}
	if len(input.Scopes) == 0 ||
		len(input.Scopes) > MaximumProfileScopes {
		return Request{}, fmt.Errorf(
			"profile_prepare scopes must contain 1..%d entries",
			MaximumProfileScopes,
		)
	}
	scopes := append([]Scope{}, input.Scopes...)
	scopeIDs := make([]string, len(scopes))
	for index, scope := range scopes {
		if scope.ScopeID() == "" {
			return Request{}, fmt.Errorf(
				"profile_prepare scope %d is invalid",
				index,
			)
		}
		scopeIDs[index] = scope.ScopeID()
	}
	slices.Sort(scopeIDs)
	if len(slices.Compact(scopeIDs)) != len(scopes) {
		return Request{}, fmt.Errorf(
			"profile_prepare scope_id values must not repeat",
		)
	}
	return Request{
		action: action,
		basis:  input.Basis,
		scopes: scopes,
	}, nil
}

func (request Request) Action() Action {
	return request.action
}

func (request Request) HasExplicitScopes() bool {
	return len(request.scopes) > 0
}

func (request Request) Basis() string {
	return request.basis
}

func (request Request) Scopes() []Scope {
	return append([]Scope{}, request.scopes...)
}

type Observation struct {
	initialized             bool
	profileDeclared         bool
	profileReviewReady      bool
	memoryReady             bool
	memoryReviewReady       bool
	memoryDeferred          bool
	detectionNeedsHelp      bool
	autoBootstrapEligible   bool
	profileOverrideEligible bool
	profileOrigin           projectprofile.ProfileAdmissionOrigin
	scopes                  []Scope
	detail                  string
}

type ObservationInput struct {
	Initialized             bool
	ProfileDeclared         bool
	ProfileReviewReady      bool
	MemoryReady             bool
	MemoryReviewReady       bool
	MemoryDeferred          bool
	DetectionNeedsHelp      bool
	AutoBootstrapEligible   bool
	ProfileOverrideEligible bool
	ProfileOrigin           projectprofile.ProfileAdmissionOrigin
	Scopes                  []Scope
	Detail                  string
}

func NewObservation(input ObservationInput) (Observation, error) {
	profileOrigin := input.ProfileOrigin
	if input.ProfileDeclared && profileOrigin == "" {
		profileOrigin = projectprofile.ProfileAdmissionOriginLegacyUnknown
	}
	if !input.ProfileDeclared && profileOrigin != "" {
		return Observation{}, fmt.Errorf(
			"profile origin requires a declared project profile",
		)
	}
	if input.ProfileDeclared && input.AutoBootstrapEligible {
		return Observation{}, fmt.Errorf(
			"automatic initial profile bootstrap cannot replace a declared profile",
		)
	}
	if input.ProfileOverrideEligible && !input.ProfileDeclared {
		return Observation{}, fmt.Errorf(
			"profile override eligibility requires a declared project profile",
		)
	}
	if input.ProfileOverrideEligible &&
		profileOrigin != projectprofile.ProfileAdmissionOriginDetectorDefault {
		return Observation{}, fmt.Errorf(
			"profile override eligibility requires detector_default origin",
		)
	}
	if input.ProfileDeclared {
		_, valid := projectprofile.ParseProfileAdmissionOrigin(string(profileOrigin))
		if !valid {
			return Observation{}, fmt.Errorf("declared project profile origin is invalid")
		}
	}
	if !input.Initialized &&
		(input.ProfileDeclared ||
			input.ProfileReviewReady ||
			input.MemoryReady ||
			input.MemoryReviewReady ||
			input.MemoryDeferred) {
		return Observation{}, fmt.Errorf(
			"uninitialized onboarding state cannot carry project setup state",
		)
	}
	if input.ProfileDeclared && input.ProfileReviewReady {
		return Observation{}, fmt.Errorf(
			"declared profile cannot remain a pending initial profile review",
		)
	}
	if !input.ProfileDeclared &&
		(input.MemoryReady ||
			input.MemoryReviewReady ||
			input.MemoryDeferred) {
		return Observation{}, fmt.Errorf(
			"structured-memory setup requires a declared project profile",
		)
	}
	memoryStates := 0
	for _, active := range []bool{
		input.MemoryReady,
		input.MemoryReviewReady,
		input.MemoryDeferred,
	} {
		if active {
			memoryStates++
		}
	}
	if memoryStates > 1 {
		return Observation{}, fmt.Errorf(
			"structured-memory readiness states are mutually exclusive",
		)
	}
	return Observation{
		initialized:             input.Initialized,
		profileDeclared:         input.ProfileDeclared,
		profileReviewReady:      input.ProfileReviewReady,
		memoryReady:             input.MemoryReady,
		memoryReviewReady:       input.MemoryReviewReady,
		memoryDeferred:          input.MemoryDeferred,
		detectionNeedsHelp:      input.DetectionNeedsHelp,
		autoBootstrapEligible:   input.AutoBootstrapEligible,
		profileOverrideEligible: input.ProfileOverrideEligible,
		profileOrigin:           profileOrigin,
		scopes:                  append([]Scope{}, input.Scopes...),
		detail:                  strings.TrimSpace(input.Detail),
	}, nil
}

func (observation Observation) Initialized() bool {
	return observation.initialized
}

func (observation Observation) ProfileDeclared() bool {
	return observation.profileDeclared
}

func (observation Observation) ProfileReviewReady() bool {
	return observation.profileReviewReady
}

func (observation Observation) MemoryReady() bool {
	return observation.memoryReady
}

func (observation Observation) MemoryReviewReady() bool {
	return observation.memoryReviewReady
}

func (observation Observation) MemoryDeferred() bool {
	return observation.memoryDeferred
}

func (observation Observation) DetectionNeedsHelp() bool {
	return observation.detectionNeedsHelp
}

func (observation Observation) AutoBootstrapEligible() bool {
	return observation.autoBootstrapEligible
}

func (observation Observation) ProfileOverrideEligible() bool {
	return observation.profileOverrideEligible
}

func (observation Observation) ProfileOrigin() projectprofile.ProfileAdmissionOrigin {
	return observation.profileOrigin
}

func (observation Observation) Scopes() []Scope {
	return append([]Scope{}, observation.scopes...)
}

func (observation Observation) Detail() string {
	return observation.detail
}

type PreparationKind string

const (
	PreparationCreated          PreparationKind = "created"
	PreparationReused           PreparationKind = "reused"
	PreparationNeedsScopeReview PreparationKind = "needs_scope_review"
	PreparationBlocked          PreparationKind = "blocked"
)

type Preparation struct {
	kind      PreparationKind
	reviewRef string
	scopes    []Scope
	detail    string
}

func NewPreparation(
	kind PreparationKind,
	reviewRef string,
	scopes []Scope,
	detail string,
) (Preparation, error) {
	if kind != PreparationCreated &&
		kind != PreparationReused &&
		kind != PreparationNeedsScopeReview &&
		kind != PreparationBlocked {
		return Preparation{}, fmt.Errorf(
			"unsupported onboarding preparation result %q",
			kind,
		)
	}
	if (kind == PreparationCreated || kind == PreparationReused) &&
		strings.TrimSpace(reviewRef) == "" {
		return Preparation{}, fmt.Errorf(
			"successful review preparation requires review_ref",
		)
	}
	return Preparation{
		kind:      kind,
		reviewRef: strings.TrimSpace(reviewRef),
		scopes:    append([]Scope{}, scopes...),
		detail:    strings.TrimSpace(detail),
	}, nil
}

func (preparation Preparation) Kind() PreparationKind {
	return preparation.kind
}

func (preparation Preparation) ReviewRef() string {
	return preparation.reviewRef
}

func (preparation Preparation) Scopes() []Scope {
	return append([]Scope{}, preparation.scopes...)
}

func (preparation Preparation) Detail() string {
	return preparation.detail
}

type Effects struct {
	RepositoryInspected     bool
	ReviewCarrierCreated    bool
	ReviewCarrierReused     bool
	CanonicalProfileChanged bool
	StructuredMemoryEnabled bool
	AuthorityGranted        bool
}

type Outcome struct {
	action                  Action
	result                  Result
	status                  Status
	detail                  string
	nextAction              string
	reviewRef               string
	autoBootstrapEligible   bool
	profileOverrideEligible bool
	profileOrigin           projectprofile.ProfileAdmissionOrigin
	scopes                  []Scope
	choices                 []string
	effects                 Effects
}

func (outcome Outcome) Action() Action {
	return outcome.action
}

func (outcome Outcome) Result() Result {
	return outcome.result
}

func (outcome Outcome) Status() Status {
	return outcome.status
}

func (outcome Outcome) Detail() string {
	return outcome.detail
}

func (outcome Outcome) NextAction() string {
	return outcome.nextAction
}

func (outcome Outcome) ReviewRef() string {
	return outcome.reviewRef
}

func (outcome Outcome) AutoBootstrapEligible() bool {
	return outcome.autoBootstrapEligible
}

func (outcome Outcome) ProfileOverrideEligible() bool {
	return outcome.profileOverrideEligible
}

func (outcome Outcome) ProfileOrigin() projectprofile.ProfileAdmissionOrigin {
	return outcome.profileOrigin
}

func (outcome Outcome) Scopes() []Scope {
	return append([]Scope{}, outcome.scopes...)
}

func (outcome Outcome) Choices() []string {
	return append([]string{}, outcome.choices...)
}

func (outcome Outcome) Effects() Effects {
	return outcome.effects
}

type Runtime interface {
	Observe(context.Context) (Observation, error)
	PrepareProfile(context.Context, Request) (Preparation, error)
	PrepareMemory(context.Context) (Preparation, error)
}

type Service struct {
	runtime              Runtime
	memoryReadyAtStartup bool
}

func NewService(
	runtime Runtime,
	memoryReadyAtStartup bool,
) (Service, error) {
	if runtime == nil {
		return Service{}, fmt.Errorf(
			"onboarding service requires a runtime",
		)
	}
	return Service{
		runtime:              runtime,
		memoryReadyAtStartup: memoryReadyAtStartup,
	}, nil
}

func (service Service) Execute(
	ctx context.Context,
	request Request,
) (Outcome, error) {
	if service.runtime == nil {
		return Outcome{}, fmt.Errorf(
			"onboarding service runtime is unavailable",
		)
	}
	observation, err := service.runtime.Observe(ctx)
	if err != nil {
		return Outcome{}, err
	}
	if !observation.Initialized() {
		return onboardingRequiredOutcome(request.Action()), nil
	}
	if !service.memoryReadyAtStartup && observation.MemoryReady() {
		return restartRequiredOutcome(request.Action(), observation.ProfileOrigin()), nil
	}
	switch request.Action() {
	case ActionStatus:
		return statusOutcome(observation), nil
	case ActionProfilePrepare:
		return service.prepareProfile(ctx, request, observation)
	case ActionMemoryPrepare:
		return service.prepareMemory(ctx, observation)
	default:
		return Outcome{}, fmt.Errorf(
			"unsupported onboarding action %q",
			request.Action(),
		)
	}
}

func (service Service) prepareProfile(
	ctx context.Context,
	request Request,
	observation Observation,
) (Outcome, error) {
	if observation.ProfileDeclared() && !observation.ProfileOverrideEligible() {
		current := statusOutcome(observation)
		return blockedOutcome(
			ActionProfilePrepare,
			current.Status(),
			"The canonical project profile already exists; initial profile preparation cannot replace it.",
			current.NextAction(),
			current.Scopes(),
		).withProfileState(
			current.ProfileOrigin(),
			current.ProfileOverrideEligible(),
		), nil
	}
	preparation, err := service.runtime.PrepareProfile(ctx, request)
	if err != nil {
		return Outcome{}, err
	}
	switch preparation.Kind() {
	case PreparationCreated:
		return preparedOutcome(
			ActionProfilePrepare,
			ResultProfileReviewCreated,
			StatusProfileReviewReady,
			preparation,
		).withProfileState(
			observation.ProfileOrigin(),
			observation.ProfileOverrideEligible(),
		), nil
	case PreparationReused:
		return preparedOutcome(
			ActionProfilePrepare,
			ResultProfileReviewReused,
			StatusProfileReviewReady,
			preparation,
		).withProfileState(
			observation.ProfileOrigin(),
			observation.ProfileOverrideEligible(),
		), nil
	case PreparationNeedsScopeReview:
		return Outcome{
			action:     ActionProfilePrepare,
			result:     ResultNeedsScopeReview,
			status:     StatusNeedsProfile,
			detail:     preparation.Detail(),
			nextAction: "Provide a readable basis and one or more explicit software or non-software scopes, then repeat profile_prepare.",
			scopes:     preparation.Scopes(),
			effects: Effects{
				RepositoryInspected: true,
			},
		}.withProfileState(
			observation.ProfileOrigin(),
			observation.ProfileOverrideEligible(),
		), nil
	case PreparationBlocked:
		return blockedOutcome(
			ActionProfilePrepare,
			StatusNeedsProfile,
			preparation.Detail(),
			"Inspect the existing readable profile review and current repository scope before retrying.",
			preparation.Scopes(),
		).withProfileState(
			observation.ProfileOrigin(),
			observation.ProfileOverrideEligible(),
		), nil
	default:
		return Outcome{}, fmt.Errorf(
			"unsupported profile preparation result %q",
			preparation.Kind(),
		)
	}
}

func (service Service) prepareMemory(
	ctx context.Context,
	observation Observation,
) (Outcome, error) {
	if !observation.ProfileDeclared() {
		return blockedOutcome(
			ActionMemoryPrepare,
			StatusNeedsProfile,
			"A canonical project profile is required before the structured-memory choice can be prepared.",
			"Prepare and explicitly apply the reviewed project profile first.",
			observation.Scopes(),
		), nil
	}
	if observation.MemoryReady() {
		return Outcome{
			action:        ActionMemoryPrepare,
			result:        ResultReady,
			status:        StatusReady,
			detail:        "Structured project memory is already enabled for this process.",
			nextAction:    "Continue with the current project question.",
			scopes:        observation.Scopes(),
			profileOrigin: observation.ProfileOrigin(),
			effects: Effects{
				RepositoryInspected: true,
			},
		}, nil
	}
	preparation, err := service.runtime.PrepareMemory(ctx)
	if err != nil {
		return Outcome{}, err
	}
	switch preparation.Kind() {
	case PreparationCreated:
		return preparedOutcome(
			ActionMemoryPrepare,
			ResultMemoryReviewCreated,
			StatusMemoryReviewReady,
			preparation,
		).withProfileOrigin(observation.ProfileOrigin()), nil
	case PreparationReused:
		return preparedOutcome(
			ActionMemoryPrepare,
			ResultMemoryReviewReused,
			StatusMemoryReviewReady,
			preparation,
		).withProfileOrigin(observation.ProfileOrigin()), nil
	case PreparationBlocked:
		status := StatusNeedsMemory
		nextAction := "Inspect the existing structured-memory review and retry only after its conflict or stale basis is resolved."
		if observation.MemoryDeferred() {
			status = StatusMemoryDeferred
			nextAction = "Inspect the recorded non-binding disposition and retry reopening only after its carrier can be read safely."
		}
		return blockedOutcome(
			ActionMemoryPrepare,
			status,
			preparation.Detail(),
			nextAction,
			observation.Scopes(),
		).withProfileOrigin(observation.ProfileOrigin()), nil
	default:
		return Outcome{}, fmt.Errorf(
			"unsupported memory preparation result %q",
			preparation.Kind(),
		)
	}
}

// StatusForObservation is the pure readable projection used by both the MCP
// status action and task-level CLI effect receipts. Keeping one projection
// prevents replay receipts from hard-coding a stale next state.
func StatusForObservation(observation Observation) Outcome {
	return statusOutcome(observation)
}

func statusOutcome(observation Observation) Outcome {
	effects := Effects{
		RepositoryInspected: true,
	}
	if !observation.ProfileDeclared() {
		if observation.AutoBootstrapEligible() {
			return Outcome{
				action:                ActionStatus,
				result:                ResultNeedsProfile,
				status:                StatusNeedsProfile,
				detail:                readableDetail(observation.Detail(), "The project has no canonical profile and satisfies the automatic supported-singleton init policy."),
				nextAction:            "Run haft init --core-only to admit the detector_default profile, install applicable carriers, and repeat status.",
				scopes:                observation.Scopes(),
				autoBootstrapEligible: true,
				effects:               effects,
			}
		}
		if observation.ProfileReviewReady() {
			return Outcome{
				action:     ActionStatus,
				result:     ResultProfileReviewReady,
				status:     StatusProfileReviewReady,
				detail:     readableDetail(observation.Detail(), "A non-binding project-profile review is ready."),
				nextAction: "Review the readable scopes; profile application requires one direct, unambiguous operator selection.",
				reviewRef:  "review:onboard-profile",
				scopes:     observation.Scopes(),
				effects:    effects,
			}
		}
		nextAction := "Call profile_prepare to create a non-binding project-profile review."
		if observation.DetectionNeedsHelp() {
			nextAction = "Call profile_prepare, then provide a readable basis and explicit scopes when requested."
		}
		return Outcome{
			action:     ActionStatus,
			result:     ResultNeedsProfile,
			status:     StatusNeedsProfile,
			detail:     readableDetail(observation.Detail(), "The project has no canonical profile."),
			nextAction: nextAction,
			scopes:     observation.Scopes(),
			effects:    effects,
		}
	}
	if observation.MemoryReady() {
		return Outcome{
			action:                  ActionStatus,
			result:                  ResultReady,
			status:                  StatusReady,
			detail:                  readableDetail(observation.Detail(), "Project profile and structured project memory are ready."),
			nextAction:              "Continue with the current project question.",
			scopes:                  observation.Scopes(),
			profileOrigin:           observation.ProfileOrigin(),
			profileOverrideEligible: observation.ProfileOverrideEligible(),
			effects:                 effects,
		}
	}
	if observation.MemoryDeferred() {
		return Outcome{
			action:                  ActionStatus,
			result:                  ResultOnboardingRequired,
			status:                  StatusNeedsInit,
			detail:                  readableDetail(observation.Detail(), "Default project memory is incomplete."),
			nextAction:              "Run haft init to repair default project memory, reload the host integration, and repeat status.",
			scopes:                  observation.Scopes(),
			profileOrigin:           observation.ProfileOrigin(),
			profileOverrideEligible: observation.ProfileOverrideEligible(),
			effects:                 effects,
		}
	}
	if observation.MemoryReviewReady() {
		return Outcome{
			action:                  ActionStatus,
			result:                  ResultOnboardingRequired,
			status:                  StatusNeedsInit,
			detail:                  readableDetail(observation.Detail(), "Default project memory is incomplete."),
			nextAction:              "Run haft init to repair default project memory, reload the host integration, and repeat status.",
			scopes:                  observation.Scopes(),
			profileOrigin:           observation.ProfileOrigin(),
			profileOverrideEligible: observation.ProfileOverrideEligible(),
			effects:                 effects,
		}
	}
	return Outcome{
		action:                  ActionStatus,
		result:                  ResultOnboardingRequired,
		status:                  StatusNeedsInit,
		detail:                  readableDetail(observation.Detail(), "Default project memory is incomplete."),
		nextAction:              "Run haft init to repair default project memory, reload the host integration, and repeat status.",
		scopes:                  observation.Scopes(),
		profileOrigin:           observation.ProfileOrigin(),
		profileOverrideEligible: observation.ProfileOverrideEligible(),
		effects:                 effects,
	}
}

func onboardingRequiredOutcome(action Action) Outcome {
	return Outcome{
		action:     action,
		result:     ResultOnboardingRequired,
		status:     StatusNeedsInit,
		detail:     "Haft is not initialized for this project; no setup state was changed.",
		nextAction: "Run haft init, reload the host integration, and call haft_onboard with action status.",
		effects:    Effects{},
	}
}

func restartRequiredOutcome(
	action Action,
	origin projectprofile.ProfileAdmissionOrigin,
) Outcome {
	return Outcome{
		action:        action,
		result:        ResultRestartRequired,
		status:        StatusReady,
		detail:        "Structured project memory became enabled after this MCP process started; this stale process made no setup change.",
		nextAction:    "Reload the host integration and call haft_onboard with action status in the new process.",
		profileOrigin: origin,
		effects: Effects{
			RepositoryInspected: true,
		},
	}
}

func (outcome Outcome) withProfileOrigin(
	origin projectprofile.ProfileAdmissionOrigin,
) Outcome {
	outcome.profileOrigin = origin
	return outcome
}

func (outcome Outcome) withProfileState(
	origin projectprofile.ProfileAdmissionOrigin,
	overrideEligible bool,
) Outcome {
	outcome.profileOrigin = origin
	outcome.profileOverrideEligible = overrideEligible
	return outcome
}

func preparedOutcome(
	action Action,
	result Result,
	status Status,
	preparation Preparation,
) Outcome {
	created := preparation.Kind() == PreparationCreated
	reused := preparation.Kind() == PreparationReused
	outcome := Outcome{
		action:    action,
		result:    result,
		status:    status,
		detail:    preparation.Detail(),
		reviewRef: preparation.ReviewRef(),
		scopes:    preparation.Scopes(),
		effects: Effects{
			RepositoryInspected:  true,
			ReviewCarrierCreated: created,
			ReviewCarrierReused:  reused,
		},
	}
	if action == ActionProfilePrepare {
		outcome.nextAction = "Review the readable scopes; profile application requires one direct, unambiguous operator selection."
		return outcome
	}
	outcome.nextAction = "Present the enable/defer choice; enabling remains an explicit h-decide act."
	outcome.choices = memoryChoices()
	return outcome
}

func blockedOutcome(
	action Action,
	status Status,
	detail string,
	nextAction string,
	scopes []Scope,
) Outcome {
	return Outcome{
		action:     action,
		result:     ResultBlocked,
		status:     status,
		detail:     strings.TrimSpace(detail),
		nextAction: strings.TrimSpace(nextAction),
		scopes:     append([]Scope{}, scopes...),
		effects: Effects{
			RepositoryInspected: true,
		},
	}
}

func BlockedOutcome(
	action Action,
	status Status,
	detail string,
	nextAction string,
) Outcome {
	return blockedOutcome(
		action,
		status,
		detail,
		nextAction,
		nil,
	)
}

func memoryChoices() []string {
	return []string{
		EnableStructuredMemoryChoice,
		DeferStructuredMemoryChoice,
	}
}

func readableDetail(observed string, fallback string) string {
	if strings.TrimSpace(observed) == "" {
		return fallback
	}
	return strings.TrimSpace(observed)
}

func requireExactReadableText(
	field string,
	value string,
	maximumBytes int,
) error {
	if value == "" ||
		value != strings.TrimSpace(value) ||
		len([]byte(value)) > maximumBytes ||
		strings.ContainsRune(value, '\x00') {
		return fmt.Errorf(
			"%s must be exact non-empty readable text of at most %d bytes",
			field,
			maximumBytes,
		)
	}
	return nil
}
