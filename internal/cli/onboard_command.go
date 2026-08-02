package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/onboarding"
	"github.com/m0n0x41d/haft/internal/onboardingfs"
	"github.com/m0n0x41d/haft/internal/profileonboarding"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projecttypeenvreviewcarrier"
	"github.com/spf13/cobra"
)

var onboardProfileApplyJSON bool
var onboardMemoryEnableJSON bool
var onboardMemoryDeferJSON bool

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Apply reviewed Haft setup choices",
	Long: `Apply one readable, already-prepared Haft onboarding choice.

Preparation remains available through haft_onboard in MCP. These CLI commands
consume the current non-binding review carrier and derive every internal
coordinate themselves. They never ask the operator for an internal schema,
staging coordinate, digest, or hidden implementation selector.`,
}

var onboardProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Apply the reviewed project profile",
}

var onboardProfileApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply the current reviewed project profile",
	Long: `Apply the current non-binding project-profile review.

The host calls this internal effect sink only after the operator directly and
unambiguously selects this exact reviewed profile and scope. The command reads
the prepared review, records host_routed_operator_request, and derives the
durable admission inputs itself; no skill name or second confirmation is
required.`,
	Args: cobra.NoArgs,
	RunE: runOnboardProfileApply,
}

var onboardMemoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Apply the reviewed structured-memory choice",
}

var onboardMemoryEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable the current reviewed structured-memory capability",
	Long: `Enable structured project memory from the current readable review.

This command is an internal effect sink after the host routes a direct,
unambiguous operator request for this exact reviewed successor. It revalidates
and consumes that review, records host_routed_operator_request, and derives all
internal effect inputs itself. It does not create a DecisionRecord or authorize
any other work.`,
	Args: cobra.NoArgs,
	RunE: runOnboardMemoryEnable,
}

var onboardMemoryDeferCmd = &cobra.Command{
	Use:   "defer",
	Short: "Record the explicit non-binding Not now choice",
	Long: `Record "Not now" for the current structured-memory review.

This is a non-binding onboarding state, not a decision record or authority
grant. A later explicit memory_prepare reopens the choice.`,
	Args: cobra.NoArgs,
	RunE: runOnboardMemoryDefer,
}

type onboardTaskEffectResponse struct {
	Kind       string                   `json:"kind"`
	Action     string                   `json:"action"`
	Result     string                   `json:"result"`
	Status     string                   `json:"status"`
	Detail     string                   `json:"detail"`
	NextAction string                   `json:"next_action"`
	ReviewRef  string                   `json:"review_ref"`
	Delivery   string                   `json:"delivery"`
	Scopes     []onboardScopeWire       `json:"scopes,omitempty"`
	Effects    onboardTaskEffectEffects `json:"effects"`
}

type onboardTaskEffectEffects struct {
	// Every field describes a new effect from this invocation. Current state is
	// reported separately through Status; exact replay keeps these fields false.
	CanonicalProfileChanged bool `json:"canonical_profile_changed"`
	StructuredMemoryEnabled bool `json:"structured_memory_enabled"`
	MemoryDeferred          bool `json:"memory_deferred"`
	DecisionRecordCreated   bool `json:"decision_record_created"`
	AuthorityGranted        bool `json:"authority_granted"`
}

type reviewedMemorySelectionEffect func(
	context.Context,
) (projectTypeEnvGenesisSelectionResponse, error)

type reviewedMemoryDeferralEffect func(
	string,
	onboarding.MemoryDeferral,
) (onboardingfs.InstallResult, error)

func init() {
	onboardProfileApplyCmd.Flags().BoolVar(
		&onboardProfileApplyJSON,
		"json",
		false,
		"print the task-level result as structured JSON",
	)
	onboardMemoryEnableCmd.Flags().BoolVar(
		&onboardMemoryEnableJSON,
		"json",
		false,
		"print the task-level result as structured JSON",
	)
	onboardMemoryDeferCmd.Flags().BoolVar(
		&onboardMemoryDeferJSON,
		"json",
		false,
		"print the task-level result as structured JSON",
	)
	onboardProfileCmd.AddCommand(onboardProfileApplyCmd)
	onboardCmd.AddCommand(
		onboardProfileCmd,
	)
	rootCmd.AddCommand(onboardCmd)
}

func runOnboardProfileApply(
	cmd *cobra.Command,
	_ []string,
) error {
	binding, err := resolveOnboardTaskBinding(
		commandContext(cmd),
	)
	if err != nil {
		return fmt.Errorf(
			"resolve exact Haft project binding for profile application: current project attachment could not be verified",
		)
	}
	declaration, effectErr := executeReviewedProfileDeclaration(
		commandContext(cmd),
		binding.ProjectRoot,
		"",
	)
	if declaration.Kind == "" {
		return fmt.Errorf(
			"apply reviewed project profile: the current review could not be verified; inspect onboarding status and prepare a fresh review when required",
		)
	}
	if declaration.Admission == nil {
		return writeOnboardProfileNoCommit(cmd, declaration)
	}
	observation, err := newProjectOnboardingRuntime(binding).Observe(
		commandContext(cmd),
	)
	if err != nil {
		response := onboardProfileUnverifiedResponse(
			declaration,
			"profile_applied_status_unverified",
			"The reviewed profile admission was recorded, but its current project status could not be verified.",
		)
		writeErr := writeOnboardTaskEffect(
			cmd.OutOrStdout(),
			response,
			onboardProfileApplyJSON,
		)
		return errors.Join(
			writeErr,
			fmt.Errorf(
				"project profile may be canonical; inspect fresh onboarding status before retrying",
			),
		)
	}
	if !observation.ProfileDeclared() {
		response := onboardProfileUnverifiedResponse(
			declaration,
			"profile_applied_status_unverified",
			"The reviewed profile admission was recorded, but no canonical profile was observable on the current project attachment.",
		)
		writeErr := writeOnboardTaskEffect(
			cmd.OutOrStdout(),
			response,
			onboardProfileApplyJSON,
		)
		return errors.Join(
			writeErr,
			fmt.Errorf(
				"project profile admission needs fresh status verification",
			),
		)
	}
	current := onboarding.StatusForObservation(observation)
	result := "profile_applied"
	detail := "The reviewed project profile is now the canonical project basis."
	attention := false
	switch declaration.State {
	case string(profileonboarding.ResultSynchronized):
		if effectErr != nil {
			result = "profile_applied_attachment_unverified"
			detail = "The reviewed project profile is canonical, but the command could not verify the final project attachment."
			attention = true
		}
	case string(profileonboarding.ResultProjectionDebt):
		result = "profile_applied_with_projection_debt"
		detail = "The reviewed project profile is canonical, but its readable file projection still needs repair."
		attention = true
	case string(profileonboarding.ResultProjectionFailed):
		result = "profile_applied_with_projection_failure"
		detail = "The reviewed project profile is canonical, but its readable file projection failed and needs repair."
		attention = true
	default:
		return fmt.Errorf(
			"apply reviewed project profile: committed result could not be projected safely",
		)
	}
	response := onboardTaskEffectResponse{
		Kind:       "haft_onboard_profile_apply",
		Action:     "profile_apply",
		Result:     result,
		Status:     string(current.Status()),
		Detail:     detail,
		NextAction: current.NextAction(),
		ReviewRef:  "review:onboard-profile",
		Delivery:   onboardProfileDelivery(declaration),
		Scopes:     onboardScopesToWire(observation.Scopes()),
		Effects: onboardTaskEffectEffects{
			CanonicalProfileChanged: declaration.Admission.Delivery == "fresh",
		},
	}
	writeErr := writeOnboardTaskEffect(
		cmd.OutOrStdout(),
		response,
		onboardProfileApplyJSON,
	)
	if attention {
		return errors.Join(
			writeErr,
			fmt.Errorf(
				"project profile is canonical but requires the reported projection or attachment repair",
			),
		)
	}
	return writeErr
}

func writeOnboardProfileNoCommit(
	cmd *cobra.Command,
	declaration profileOnboardResponse,
) error {
	result := "blocked_without_commit"
	status := "blocked"
	detail := "The reviewed project profile was not applied."
	nextAction := "Inspect the current readable review and repository evidence before retrying."
	delivery := "none"
	if declaration.Failure != nil &&
		declaration.Failure.CommitPosture ==
			"commit_outcome_unknown" {
		result = "commit_outcome_unknown"
		status = "outcome_unknown"
		detail = "The command could not determine whether the exact reviewed profile application committed."
		nextAction = "Retry the unchanged reviewed profile; do not replace its review until the exact outcome is resolved."
		delivery = "outcome_unknown"
	}
	response := onboardTaskEffectResponse{
		Kind:       "haft_onboard_profile_apply",
		Action:     "profile_apply",
		Result:     result,
		Status:     status,
		Detail:     detail,
		NextAction: nextAction,
		ReviewRef:  "review:onboard-profile",
		Delivery:   delivery,
		Effects:    onboardTaskEffectEffects{},
	}
	writeErr := writeOnboardTaskEffect(
		cmd.OutOrStdout(),
		response,
		onboardProfileApplyJSON,
	)
	return errors.Join(
		writeErr,
		fmt.Errorf(
			"reviewed project profile was not confirmed as canonical",
		),
	)
}

func onboardProfileUnverifiedResponse(
	declaration profileOnboardResponse,
	result string,
	detail string,
) onboardTaskEffectResponse {
	return onboardTaskEffectResponse{
		Kind:       "haft_onboard_profile_apply",
		Action:     "profile_apply",
		Result:     result,
		Status:     "verification_required",
		Detail:     detail,
		NextAction: "Read fresh onboarding status before retrying the unchanged reviewed application.",
		ReviewRef:  "review:onboard-profile",
		Delivery:   onboardProfileDelivery(declaration),
		Effects: onboardTaskEffectEffects{
			CanonicalProfileChanged: declaration.Admission != nil &&
				declaration.Admission.Delivery == "fresh",
		},
	}
}

func onboardProfileDelivery(
	declaration profileOnboardResponse,
) string {
	if declaration.Admission == nil {
		return "none"
	}
	if declaration.Admission.Delivery == "fresh" {
		return "applied"
	}
	return "reused"
}

func runOnboardMemoryEnable(
	cmd *cobra.Command,
	_ []string,
) error {
	return runOnboardMemoryEnableWithEffect(
		cmd,
		executeReviewedMemorySelection,
	)
}

func runOnboardMemoryEnableWithEffect(
	cmd *cobra.Command,
	selectEffect reviewedMemorySelectionEffect,
) error {
	if selectEffect == nil {
		return fmt.Errorf(
			"structured-memory enablement effect is unavailable",
		)
	}
	binding, err := resolveOnboardTaskBinding(
		commandContext(cmd),
	)
	if err != nil {
		return fmt.Errorf(
			"resolve exact Haft project binding for memory enablement: current project attachment could not be verified",
		)
	}
	runtime := newProjectOnboardingRuntime(binding)
	before, err := runtime.Observe(commandContext(cmd))
	if err != nil {
		return fmt.Errorf(
			"inspect structured-memory review: current onboarding state could not be read safely",
		)
	}
	if !before.MemoryReady() && before.MemoryDeferred() {
		return fmt.Errorf(
			"structured memory is deferred; call haft_onboard memory_prepare to reopen the choice",
		)
	}
	if !before.MemoryReady() && !before.MemoryReviewReady() {
		return fmt.Errorf(
			"no current structured-memory review is ready; call haft_onboard memory_prepare first",
		)
	}
	selection, effectErr := selectEffect(
		commandContext(cmd),
	)
	if selection.ContractVersion == "" {
		return fmt.Errorf(
			"enable structured project memory: the current reviewed effect could not be verified; inspect onboarding status before retrying",
		)
	}
	changed := false
	delivery := ""
	switch selection.Outcome.(type) {
	case projectTypeEnvGenesisFreshlyCommitted:
		changed = true
		delivery = "applied"
	case projectTypeEnvGenesisReplayedExisting:
		delivery = "reused"
	default:
		return writeOnboardMemoryNoCommit(
			cmd,
			before,
			selection.Outcome,
		)
	}
	_, verified := selection.PostEffectLedgerRevalidation.(projectTypeEnvGenesisLedgerVerifiedAfterEffect)
	if !verified || effectErr != nil {
		response := onboardMemoryEnableUnverifiedResponse(
			before.Scopes(),
			delivery,
			changed,
		)
		writeErr := writeOnboardTaskEffect(
			cmd.OutOrStdout(),
			response,
			onboardMemoryEnableJSON,
		)
		return errors.Join(
			writeErr,
			fmt.Errorf(
				"structured-memory effect was recorded but its current project attachment requires fresh verification",
			),
		)
	}
	cleanupErr := clearOnboardMemoryDeferral(
		binding.ProjectRoot,
		binding.ProjectID,
	)
	after, err := runtime.Observe(commandContext(cmd))
	if err != nil || !after.MemoryReady() {
		response := onboardMemoryEnableUnverifiedResponse(
			before.Scopes(),
			delivery,
			changed,
		)
		writeErr := writeOnboardTaskEffect(
			cmd.OutOrStdout(),
			response,
			onboardMemoryEnableJSON,
		)
		return errors.Join(
			writeErr,
			fmt.Errorf(
				"structured-memory effect was recorded but fresh readiness could not be observed",
			),
		)
	}
	response := onboardMemoryRestartResponse(
		after.Scopes(),
		delivery,
		changed,
	)
	if cleanupErr != nil {
		response.Detail += " A superseded non-binding disposition carrier still needs cleanup."
		writeErr := writeOnboardTaskEffect(
			cmd.OutOrStdout(),
			response,
			onboardMemoryEnableJSON,
		)
		return errors.Join(
			writeErr,
			fmt.Errorf(
				"structured memory is enabled, but a superseded non-binding disposition carrier needs cleanup",
			),
		)
	}
	return writeOnboardTaskEffect(
		cmd.OutOrStdout(),
		response,
		onboardMemoryEnableJSON,
	)
}

func writeOnboardMemoryNoCommit(
	cmd *cobra.Command,
	before onboarding.Observation,
	outcome projectTypeEnvGenesisSelectionOutcome,
) error {
	current := onboarding.StatusForObservation(before)
	response := onboardTaskEffectResponse{
		Kind:       "haft_onboard_memory_enable",
		Action:     "memory_enable",
		Status:     string(current.Status()),
		NextAction: current.NextAction(),
		ReviewRef:  onboarding.MemoryReviewRef,
		Delivery:   "none",
		Scopes:     onboardScopesToWire(before.Scopes()),
		Effects:    onboardTaskEffectEffects{},
	}
	switch value := outcome.(type) {
	case projectTypeEnvGenesisNotSelected:
		response.Result = "not_enabled"
		response.Detail, response.NextAction =
			readableMemoryNotSelected(value.Reason)
	case projectTypeEnvGenesisReplayConflict:
		response.Result = "enablement_conflict"
		response.Detail = "The exact reviewed enablement conflicts with a prior attempt; no new memory state was claimed."
		response.NextAction = "Inspect the existing attempt and retain the current review; do not retry with changed reviewed content."
		response.Delivery = "conflict"
	case projectTypeEnvGenesisCommitOutcomeUnknown:
		response.Result = "commit_outcome_unknown"
		response.Status = "outcome_unknown"
		response.Detail = "The command could not determine whether the exact reviewed enablement committed."
		response.NextAction = "Retry the unchanged reviewed enablement; do not replace or alter the review until the exact outcome is resolved."
		response.Delivery = "outcome_unknown"
	default:
		response.Result = "blocked_without_commit"
		response.Status = "blocked"
		response.Detail = "The reviewed structured-memory choice returned an unsupported no-commit result."
		response.NextAction = "Inspect fresh onboarding status before retrying."
	}
	writeErr := writeOnboardTaskEffect(
		cmd.OutOrStdout(),
		response,
		onboardMemoryEnableJSON,
	)
	return errors.Join(
		writeErr,
		fmt.Errorf(
			"structured project memory was not confirmed as enabled",
		),
	)
}

func readableMemoryNotSelected(
	reason string,
) (string, string) {
	switch reason {
	case "current_authority_rejection":
		return "The host-routed operator request did not match the current reviewed enablement.",
			"Return to the unchanged readable review and ask for one exact natural-language selection."
	case "review_expired":
		return "The structured-memory review expired before enablement.",
			"Prepare a fresh readable review and obtain a new direct operator selection."
	case "profile_incompatible",
		"profile_underdetermined",
		"profile_drift":
		return "The current project profile no longer supports this reviewed enablement.",
			"Repair or explicitly review the project profile, then prepare a fresh structured-memory review."
	case "stale_graph",
		"stage_drift",
		"assertion_revalidation_failure":
		return "The project memory basis changed after this review was prepared.",
			"Prepare a fresh readable structured-memory review before making another choice."
	case "prior_head_exists":
		return "The project already has a different structured-memory state.",
			"Read fresh onboarding status and use the current successor-review route when a change is still intended."
	default:
		return "The current reviewed structured-memory enablement was not accepted.",
			"Inspect fresh onboarding status and prepare a new review only when the current one is no longer valid."
	}
}

func onboardMemoryEnableUnverifiedResponse(
	scopes []onboarding.Scope,
	delivery string,
	changed bool,
) onboardTaskEffectResponse {
	return onboardTaskEffectResponse{
		Kind:       "haft_onboard_memory_enable",
		Action:     "memory_enable",
		Result:     "effect_recorded_attachment_unverified",
		Status:     "verification_required",
		Detail:     "The exact reviewed structured-memory effect was recorded, but current project attachment or readiness could not be verified.",
		NextAction: "Read fresh onboarding status before retrying the unchanged reviewed effect.",
		ReviewRef:  onboarding.MemoryReviewRef,
		Delivery:   delivery,
		Scopes:     onboardScopesToWire(scopes),
		Effects: onboardTaskEffectEffects{
			StructuredMemoryEnabled: changed,
		},
	}
}

func onboardMemoryRestartResponse(
	scopes []onboarding.Scope,
	delivery string,
	enabled bool,
) onboardTaskEffectResponse {
	return onboardTaskEffectResponse{
		Kind:       "haft_onboard_memory_enable",
		Action:     "memory_enable",
		Result:     "restart_required",
		Status:     "restart_required",
		Detail:     "Structured project memory is enabled. A running MCP client must reconnect before using it.",
		NextAction: "Reconnect the host integration and call haft_onboard with action status; rely on readiness only when the fresh status is ready.",
		ReviewRef:  onboarding.MemoryReviewRef,
		Delivery:   delivery,
		Scopes:     onboardScopesToWire(scopes),
		Effects: onboardTaskEffectEffects{
			StructuredMemoryEnabled: enabled,
		},
	}
}

func runOnboardMemoryDefer(
	cmd *cobra.Command,
	_ []string,
) error {
	return runOnboardMemoryDeferWithEffect(
		cmd,
		onboardingfs.Install,
	)
}

func runOnboardMemoryDeferWithEffect(
	cmd *cobra.Command,
	installEffect reviewedMemoryDeferralEffect,
) error {
	if installEffect == nil {
		return fmt.Errorf(
			"structured-memory deferral effect is unavailable",
		)
	}
	binding, err := resolveOnboardTaskBinding(
		commandContext(cmd),
	)
	if err != nil {
		return fmt.Errorf(
			"resolve exact Haft project binding for memory deferral: current project attachment could not be verified",
		)
	}
	runtime := newProjectOnboardingRuntime(binding)
	observation, err := runtime.Observe(commandContext(cmd))
	if err != nil {
		return fmt.Errorf(
			"inspect structured-memory review: current onboarding state could not be read safely",
		)
	}
	if observation.MemoryReady() {
		return fmt.Errorf(
			"structured project memory is already enabled",
		)
	}
	if observation.MemoryDeferred() {
		return writeOnboardTaskEffect(
			cmd.OutOrStdout(),
			onboardMemoryDeferredResponse(
				observation.Scopes(),
				"reused",
				false,
			),
			onboardMemoryDeferJSON,
		)
	}
	if !observation.MemoryReviewReady() {
		return fmt.Errorf(
			"no current structured-memory review is ready; call haft_onboard memory_prepare first",
		)
	}
	sealedReview, err := projecttypeenvreviewcarrier.Read(
		binding.ProjectRoot,
	)
	if err != nil {
		return fmt.Errorf(
			"read current structured-memory review: the readable review could not be verified",
		)
	}
	deferral, err := onboarding.NewMemoryDeferral(
		onboarding.MemoryDeferralInput{
			ProjectID:    binding.ProjectID,
			ReviewRef:    onboarding.MemoryReviewRef,
			ReviewDigest: sealedReview.Digest().String(),
			Choice:       onboarding.DeferStructuredMemoryChoice,
			RecordedAt:   time.Now().UTC(),
		},
	)
	if err != nil {
		return err
	}
	installation, err := installEffect(
		binding.ProjectRoot,
		deferral,
	)
	if err != nil {
		return fmt.Errorf(
			"record the non-binding Not now disposition: the current onboarding carrier could not be updated safely",
		)
	}
	delivery := ""
	changed := false
	switch installation.(type) {
	case onboardingfs.Created:
		delivery = "recorded"
		changed = true
	case onboardingfs.Reused:
		delivery = "reused"
	case onboardingfs.Conflict:
		response := onboardMemoryDeferFailureResponse(
			observation.Scopes(),
			"deferral_conflict",
			"conflict",
			"A different non-binding structured-memory disposition is already present and was retained unchanged.",
			"Inspect fresh onboarding status and the current readable review before retrying.",
		)
		writeErr := writeOnboardTaskEffect(
			cmd.OutOrStdout(),
			response,
			onboardMemoryDeferJSON,
		)
		return errors.Join(
			writeErr,
			fmt.Errorf(
				"the non-binding Not now disposition conflicted with current onboarding state",
			),
		)
	case onboardingfs.OutcomeUnknown:
		response := onboardMemoryDeferFailureResponse(
			observation.Scopes(),
			"deferral_outcome_unknown",
			"outcome_unknown",
			"The command could not determine whether the exact non-binding Not now disposition was recorded.",
			"Retry the unchanged defer command before preparing or choosing different reviewed content.",
		)
		writeErr := writeOnboardTaskEffect(
			cmd.OutOrStdout(),
			response,
			onboardMemoryDeferJSON,
		)
		return errors.Join(
			writeErr,
			fmt.Errorf(
				"the non-binding disposition outcome is unknown",
			),
		)
	default:
		return fmt.Errorf(
			"record the non-binding Not now disposition: unsupported filesystem result",
		)
	}
	after, err := runtime.Observe(commandContext(cmd))
	if err != nil {
		response := onboardMemoryDeferFailureResponse(
			observation.Scopes(),
			"deferral_recorded_status_unverified",
			"verification_required",
			"The non-binding disposition was recorded, but its current onboarding status could not be verified.",
			"Read fresh onboarding status before retrying the unchanged defer command.",
		)
		response.Delivery = delivery
		response.Effects.MemoryDeferred = changed
		writeErr := writeOnboardTaskEffect(
			cmd.OutOrStdout(),
			response,
			onboardMemoryDeferJSON,
		)
		return errors.Join(
			writeErr,
			fmt.Errorf(
				"the recorded non-binding disposition needs fresh status verification",
			),
		)
	}
	if after.MemoryReady() {
		response := onboardTaskEffectResponse{
			Kind:       "haft_onboard_memory_defer",
			Action:     "memory_defer",
			Result:     "defer_superseded_by_enabled_memory",
			Status:     "restart_required",
			Detail:     "Structured project memory became enabled while the non-binding disposition was being recorded; enabled memory takes precedence.",
			NextAction: "Reconnect the host integration and rely on readiness only when fresh onboarding status returns ready.",
			ReviewRef:  onboarding.MemoryReviewRef,
			Delivery:   delivery,
			Scopes:     onboardScopesToWire(after.Scopes()),
			Effects: onboardTaskEffectEffects{
				MemoryDeferred: changed,
			},
		}
		writeErr := writeOnboardTaskEffect(
			cmd.OutOrStdout(),
			response,
			onboardMemoryDeferJSON,
		)
		return errors.Join(
			writeErr,
			fmt.Errorf(
				"the non-binding defer disposition was superseded by enabled memory",
			),
		)
	}
	if !after.MemoryDeferred() {
		response := onboardMemoryDeferFailureResponse(
			after.Scopes(),
			"deferral_recorded_status_unverified",
			"verification_required",
			"The non-binding disposition was recorded, but it was not observable as the current onboarding state.",
			"Read fresh onboarding status before retrying the unchanged defer command.",
		)
		response.Delivery = delivery
		response.Effects.MemoryDeferred = changed
		writeErr := writeOnboardTaskEffect(
			cmd.OutOrStdout(),
			response,
			onboardMemoryDeferJSON,
		)
		return errors.Join(
			writeErr,
			fmt.Errorf(
				"the recorded non-binding disposition needs fresh status verification",
			),
		)
	}
	return writeOnboardTaskEffect(
		cmd.OutOrStdout(),
		onboardMemoryDeferredResponse(
			after.Scopes(),
			delivery,
			changed,
		),
		onboardMemoryDeferJSON,
	)
}

func onboardMemoryDeferredResponse(
	scopes []onboarding.Scope,
	delivery string,
	changed bool,
) onboardTaskEffectResponse {
	return onboardTaskEffectResponse{
		Kind:       "haft_onboard_memory_defer",
		Action:     "memory_defer",
		Result:     "memory_deferred",
		Status:     string(onboarding.StatusMemoryDeferred),
		Detail:     "The dedicated defer command recorded a non-binding Not now disposition. It is not a DecisionRecord, authority receipt, or proof of a separately observed human speech act.",
		NextAction: "Continue without structured memory, or call haft_onboard memory_prepare later to reopen the choice.",
		ReviewRef:  onboarding.MemoryReviewRef,
		Delivery:   delivery,
		Scopes:     onboardScopesToWire(scopes),
		Effects: onboardTaskEffectEffects{
			MemoryDeferred: changed,
		},
	}
}

func onboardMemoryDeferFailureResponse(
	scopes []onboarding.Scope,
	result string,
	status string,
	detail string,
	nextAction string,
) onboardTaskEffectResponse {
	return onboardTaskEffectResponse{
		Kind:       "haft_onboard_memory_defer",
		Action:     "memory_defer",
		Result:     result,
		Status:     status,
		Detail:     detail,
		NextAction: nextAction,
		ReviewRef:  onboarding.MemoryReviewRef,
		Delivery:   status,
		Scopes:     onboardScopesToWire(scopes),
		Effects:    onboardTaskEffectEffects{},
	}
}

func newProjectOnboardingRuntime(
	binding ProjectBinding,
) *projectOnboardingRuntime {
	return &projectOnboardingRuntime{
		binding: binding,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func resolveOnboardTaskBinding(
	ctx context.Context,
) (
	binding ProjectBinding,
	runErr error,
) {
	ledger, binding, err := openProjectTypeEnvGenesisLedger(
		ctx,
		projectledger.ReadOnly,
	)
	if err != nil {
		return binding, err
	}
	defer func() {
		runErr = errors.Join(runErr, ledger.Close())
	}()
	return binding, nil
}

func writeOnboardTaskEffect(
	writer io.Writer,
	response onboardTaskEffectResponse,
	asJSON bool,
) error {
	if asJSON {
		return writeIndentedJSON(writer, response)
	}
	lines := []string{
		onboardTaskEffectTitle(response),
		response.Detail,
		"Next: " + response.NextAction,
	}
	_, err := fmt.Fprintln(
		writer,
		strings.Join(lines, "\n"),
	)
	return err
}

func onboardTaskEffectTitle(
	response onboardTaskEffectResponse,
) string {
	switch response.Action {
	case "profile_apply":
		return "Haft onboarding: project profile applied"
	case "memory_enable":
		return "Haft onboarding: structured memory enabled"
	case "memory_defer":
		return "Haft onboarding: structured memory deferred"
	default:
		return "Haft onboarding: " + response.Result
	}
}

func readOnboardMemoryDeferral(
	projectRoot string,
) (onboarding.MemoryDeferral, bool, error) {
	result, err := onboardingfs.Read(projectRoot)
	if err != nil {
		return onboarding.MemoryDeferral{}, false, err
	}
	switch value := result.(type) {
	case onboardingfs.Absent:
		return onboarding.MemoryDeferral{}, false, nil
	case onboardingfs.Present:
		return value.Deferral, true, nil
	default:
		return onboarding.MemoryDeferral{}, false, fmt.Errorf(
			"memory deferral carrier returned an unsupported read state",
		)
	}
}

func clearOnboardMemoryDeferral(
	projectRoot string,
	projectID string,
) error {
	deferral, present, err :=
		readOnboardMemoryDeferral(projectRoot)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	if deferral.ProjectID() != projectID {
		return fmt.Errorf(
			"memory deferral belongs to another project and was retained",
		)
	}
	result, err := onboardingfs.Reopen(projectRoot, deferral)
	if err != nil {
		return err
	}
	switch result.(type) {
	case onboardingfs.AlreadyOpen,
		onboardingfs.Reopened:
		return nil
	case onboardingfs.ReopenConflict:
		return fmt.Errorf(
			"memory deferral changed concurrently and was retained",
		)
	case onboardingfs.ReopenOutcomeUnknown:
		return fmt.Errorf(
			"memory deferral reopen outcome is unknown; retry the unchanged reopen operation",
		)
	default:
		return fmt.Errorf(
			"memory deferral reopen returned an unsupported result",
		)
	}
}

func inspectOnboardMemoryDeferral(
	runtime *projectOnboardingRuntime,
) (bool, string) {
	deferral, present, err :=
		readOnboardMemoryDeferral(
			runtime.binding.ProjectRoot,
		)
	if err != nil {
		return false,
			"The existing Not now choice is unreadable; it was not treated as current."
	}
	if !present {
		return false, ""
	}
	if deferral.ProjectID() != runtime.binding.ProjectID ||
		deferral.ReviewRef() != onboarding.MemoryReviewRef ||
		deferral.Choice() !=
			onboarding.DeferStructuredMemoryChoice {
		return false,
			"The existing Not now choice belongs to a different onboarding basis; it was not treated as current."
	}
	review, err := projecttypeenvreviewcarrier.Read(
		runtime.binding.ProjectRoot,
	)
	if err != nil ||
		review.Digest().String() !=
			deferral.ReviewDigest() {
		return false,
			"The reviewed basis for the existing Not now choice is unavailable or changed; the choice was not treated as current."
	}
	carrier, err := readProjectTypeEnvGenesisReview(
		runtime.binding.ProjectRoot,
	)
	if err != nil {
		return false,
			"The reviewed basis for the existing Not now choice is unreadable; the choice was not treated as current."
	}
	preparedAt, preparedErr := time.Parse(
		time.RFC3339Nano,
		carrier.PreparedAt,
	)
	expiresAt, expiresErr := time.Parse(
		time.RFC3339Nano,
		carrier.ExpiresAt,
	)
	if preparedErr != nil ||
		expiresErr != nil ||
		deferral.RecordedAt().Before(preparedAt) ||
		deferral.RecordedAt().After(expiresAt) {
		return false,
			"The existing Not now choice was not recorded against the review's valid window; it was not treated as current."
	}
	return true,
		"The non-binding Not now disposition is current for the reviewed structured-memory option."
}
