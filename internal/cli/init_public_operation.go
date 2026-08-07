package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"

	"github.com/m0n0x41d/haft/internal/initexecution"
	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

type publicAgentSkillsEffectPreview struct {
	Path           string
	Kind           publicAgentSkillsEffectKind
	ExpectedDigest string
	ExpectedMode   uint32
	RenderedDigest string
	RenderedMode   uint32
}

type publicAgentSkillsPreview struct {
	Scope          initplanning.InstallScope
	Root           string
	Effects        []publicAgentSkillsEffectPreview
	ManifestPath   string
	ManifestDigest string
}

type typedPublicInitPreview struct {
	Base                   initplanning.InitPlanPreview
	Hermes                 []publicHermesPreview
	Overseer               []publicOverseerPreview
	AgentSkills            []publicAgentSkillsPreview
	DeprecatedSkillCleanup []publicDeprecatedSkillCleanupPreview
	LegacyCommandCleanup   []publicLegacyCommandCleanupPreview
}

type preparedTypedPublicInitOperation struct {
	base           initexecution.PreparedInitOperation
	hermes         publicHermesPlan
	hasHermes      bool
	overseer       publicOverseerPlan
	hasOverseer    bool
	agentSkills    publicAgentSkillsPlan
	hasAgentSkills bool
	cleanup        publicDeprecatedSkillCleanupPlan
	hasCleanup     bool
	coordinator    initfs.PublicationCoordinator
	resources      []string
	prepared       bool
}

func prepareTypedPublicInitOperation(
	ctx context.Context,
	request publicInitRequest,
	runtime currentHostPublicationRuntime,
	output io.Writer,
	maxCarrierBytes int64,
) (preparedTypedPublicInitOperation, error) {
	if output == nil {
		return preparedTypedPublicInitOperation{},
			fmt.Errorf("typed public init output is required")
	}
	core, err := compilePublicCorePlan(
		ctx,
		request,
		runtime.userHomeRoot,
	)
	if err != nil {
		return preparedTypedPublicInitOperation{}, err
	}
	var base initexecution.PreparedInitOperation
	if len(request.hostBindings) == 0 {
		plan, planErr := compilePublicCoreOnlyInitPlan(
			request,
			core,
		)
		if planErr != nil {
			return preparedTypedPublicInitOperation{}, planErr
		}
		base, err = initexecution.PrepareCoreOnlyInitOperation(plan)
	} else {
		plan, planErr := compilePublicHostInitPlan(
			request,
			core,
			runtime,
			maxCarrierBytes,
		)
		if planErr != nil {
			return preparedTypedPublicInitOperation{}, planErr
		}
		base, err = initexecution.PrepareHostInitOperation(
			plan,
			runtime.userHomeRoot,
			maxCarrierBytes,
		)
	}
	if err != nil {
		return preparedTypedPublicInitOperation{}, err
	}
	prepared := preparedTypedPublicInitOperation{
		base:     base,
		prepared: true,
	}
	if request.hermes.kind == publicHermesConfigure {
		hermesPlan, hermesErr := compilePublicHermesPlan(
			request,
			runtime,
		)
		if hermesErr != nil {
			return preparedTypedPublicInitOperation{}, hermesErr
		}
		prepared.hermes = hermesPlan
		prepared.hasHermes = true
	}
	if request.overseer.kind == publicOverseerConfigure {
		overseerPlan, overseerErr := compilePublicOverseerPlan(
			request,
		)
		if overseerErr != nil {
			return preparedTypedPublicInitOperation{}, overseerErr
		}
		prepared.overseer = overseerPlan
		prepared.hasOverseer = true
	}
	if request.agentSkills != publicAgentSkillsNone {
		bundle, bundleErr := currentSkillSourceBundle()
		if bundleErr != nil {
			return preparedTypedPublicInitOperation{}, bundleErr
		}
		agentSkills, agentSkillsErr := compilePublicAgentSkillsPlan(
			request,
			runtime.userHomeRoot,
			bundle,
		)
		if agentSkillsErr != nil {
			return preparedTypedPublicInitOperation{},
				agentSkillsErr
		}
		prepared.agentSkills = agentSkills
		prepared.hasAgentSkills = true
	}
	cleanup, err := compilePublicDeprecatedSkillCleanupPlan(
		request,
		runtime,
		prepared.hermes,
		prepared.hasHermes,
	)
	if err != nil {
		return preparedTypedPublicInitOperation{}, err
	}
	prepared.cleanup = cleanup
	prepared.hasCleanup = len(cleanup.removals) > 0
	coordinator, resources, err :=
		compileTypedPublicInitCoordination(
			request,
			runtime,
			prepared,
		)
	if err != nil {
		return preparedTypedPublicInitOperation{}, err
	}
	prepared.coordinator = coordinator
	prepared.resources = resources
	return prepared, nil
}

func compileTypedPublicInitCoordination(
	request publicInitRequest,
	runtime currentHostPublicationRuntime,
	operation preparedTypedPublicInitOperation,
) (
	initfs.PublicationCoordinator,
	[]string,
	error,
) {
	layout, err := initplanning.NewPublicationLayout(
		initplanning.PublicationLayoutInput{
			ProjectRoot:  request.projectRoot,
			ProjectID:    request.projectID,
			UserHomeRoot: runtime.userHomeRoot,
		},
	)
	if err != nil {
		return initfs.PublicationCoordinator{}, nil, err
	}
	location := layout.CoordinationLocation()
	coordinator, err := initfs.NewPublicationCoordinator(
		location.Root(),
		location.LockPath(),
	)
	if err != nil {
		return initfs.PublicationCoordinator{}, nil, err
	}
	resources, err := operation.base.PublicationResources()
	if err != nil {
		return initfs.PublicationCoordinator{}, nil, err
	}
	basePreview, err := operation.base.Preview()
	if err != nil {
		return initfs.PublicationCoordinator{}, nil, err
	}
	for _, file := range basePreview.Core.FileEffects {
		resources = append(resources, file.Path)
	}
	if basePreview.Core.RootMigrationSource != "" {
		resources = append(
			resources,
			basePreview.Core.RootMigrationSource,
			basePreview.Core.RootMigrationTarget,
		)
	}
	resources = append(
		resources,
		publicExactFilePaths(operation.hermes.effects)...,
	)
	resources = append(
		resources,
		publicExactFilePaths(operation.overseer.effects)...,
	)
	for _, effect := range operation.agentSkills.Effects() {
		resources = append(resources, effect.Path())
	}
	for _, removal := range operation.cleanup.removals {
		resources = append(resources, removal.path)
	}
	return coordinator, resources, nil
}

func (operation preparedTypedPublicInitOperation) Preview() (
	typedPublicInitPreview,
	error,
) {
	if !operation.prepared {
		return typedPublicInitPreview{},
			fmt.Errorf("typed public init operation is not prepared")
	}
	base, err := operation.base.Preview()
	if err != nil {
		return typedPublicInitPreview{}, err
	}
	preview := typedPublicInitPreview{Base: base}
	if operation.hasHermes {
		preview.Hermes = []publicHermesPreview{{
			Home:       operation.hermes.home,
			ConfigPath: operation.hermes.configPath,
			SkillsRoot: operation.hermes.skillsRoot,
			Effects: previewPublicExactFileEffects(
				operation.hermes.effects,
			),
		}}
	}
	if operation.hasOverseer {
		preview.Overseer = []publicOverseerPreview{{
			Effects: previewPublicExactFileEffects(
				operation.overseer.effects,
			),
			HookSkippedReason: operation.overseer.hookSkippedReason,
		}}
	}
	if operation.hasAgentSkills {
		effects := operation.agentSkills.Effects()
		effectPreviews := make(
			[]publicAgentSkillsEffectPreview,
			len(effects),
		)
		for index, effect := range effects {
			effectPreviews[index] = publicAgentSkillsEffectPreview{
				Path:           effect.Path(),
				Kind:           effect.kind,
				ExpectedDigest: effect.expectedDigest,
				ExpectedMode:   uint32(effect.expectedMode.Perm()),
				RenderedDigest: effect.RenderedDigest(),
				RenderedMode:   uint32(effect.output.Mode().Perm()),
			}
		}
		preview.AgentSkills = []publicAgentSkillsPreview{{
			Scope:          operation.agentSkills.scope,
			Root:           operation.agentSkills.root,
			Effects:        effectPreviews,
			ManifestPath:   operation.agentSkills.manifestPath,
			ManifestDigest: operation.agentSkills.manifestDigest,
		}}
	}
	if operation.hasCleanup {
		if publicCleanupPlanHasKind(
			operation.cleanup,
			publicDeprecatedSkillTreeRemoval,
		) {
			preview.DeprecatedSkillCleanup =
				[]publicDeprecatedSkillCleanupPreview{
					previewPublicDeprecatedSkillCleanup(
						operation.cleanup,
					),
				}
		}
		if publicCleanupPlanHasKind(
			operation.cleanup,
			publicLegacyCommandFileRemoval,
		) {
			preview.LegacyCommandCleanup =
				[]publicLegacyCommandCleanupPreview{
					previewPublicLegacyCommandCleanup(
						operation.cleanup,
					),
				}
		}
	}
	return preview, nil
}

func (operation preparedTypedPublicInitOperation) ConfirmPreview(
	reviewed typedPublicInitPreview,
) (confirmedTypedPublicInitOperation, error) {
	exact, err := operation.Preview()
	if err != nil {
		return confirmedTypedPublicInitOperation{}, err
	}
	if !reflect.DeepEqual(exact, reviewed) {
		return confirmedTypedPublicInitOperation{}, fmt.Errorf(
			"reviewed public init preview differs from the prepared operation",
		)
	}
	base, err := operation.base.ConfirmPreview(reviewed.Base)
	if err != nil {
		return confirmedTypedPublicInitOperation{}, err
	}
	return confirmedTypedPublicInitOperation{
		base:           base,
		hermes:         operation.hermes,
		hasHermes:      operation.hasHermes,
		overseer:       operation.overseer,
		hasOverseer:    operation.hasOverseer,
		agentSkills:    operation.agentSkills,
		hasAgentSkills: operation.hasAgentSkills,
		cleanup:        operation.cleanup,
		hasCleanup:     operation.hasCleanup,
		coordinator:    operation.coordinator,
		resources:      slices.Clone(operation.resources),
		confirmed:      true,
	}, nil
}

func compilePublicCoreOnlyInitPlan(
	request publicInitRequest,
	core initplanning.CoreProjectPlan,
) (initplanning.InitPlan, error) {
	intent, err := initplanning.ParseInitIntent(
		initplanning.WeakInitIntent{
			InvocationPolicy: string(request.invocation),
			ProjectRoot:      request.projectRoot,
			ProjectID:        request.projectID,
		},
	)
	if err != nil {
		return initplanning.InitPlan{}, err
	}
	return initplanning.CompileInitPlan(
		intent,
		core,
		nil,
		initplanning.AdapterCatalog{},
	)
}

type typedPublicInitExecutor struct {
	base initexecution.Executor
}

func newTypedPublicInitExecutor(
	request publicInitRequest,
	output io.Writer,
	maxCarrierBytes int64,
) (typedPublicInitExecutor, error) {
	publisher, err := initfs.NewHostPublisher(maxCarrierBytes)
	if err != nil {
		return typedPublicInitExecutor{}, err
	}
	base, err := initexecution.NewExecutor(
		newPublicProjectCoreEffect(request, output),
		publisher,
	)
	if err != nil {
		return typedPublicInitExecutor{}, err
	}
	return typedPublicInitExecutor{base: base}, nil
}

type publicInitOutcomeKind string

const (
	publicInitApplied               publicInitOutcomeKind = "applied"
	publicInitAlreadyCurrent        publicInitOutcomeKind = "already_current"
	publicInitPublicationIncomplete publicInitOutcomeKind = "publication_incomplete"
	publicInitPlanBlocked           publicInitOutcomeKind = "plan_blocked"
	publicInitBusy                  publicInitOutcomeKind = "busy"
	publicInitCoordinationFailure   publicInitOutcomeKind = "coordination_unavailable"
	publicInitCoreUnconfirmed       publicInitOutcomeKind = "core_effect_unconfirmed"
)

type typedPublicInitOutcome struct {
	kind           publicInitOutcomeKind
	base           initexecution.InitExecutionOutcome
	hermes         publicExactFileReceipt
	hasHermes      bool
	overseer       publicExactFileReceipt
	hasOverseer    bool
	agentSkills    publicAgentSkillsReceipt
	hasAgentSkills bool
	cleanup        publicExactFileReceipt
	cleanupPlan    publicDeprecatedSkillCleanupPlan
	hasCleanup     bool
	coreEffects    publicExactFileReceipt
	hasCoreEffects bool
}

func (outcome typedPublicInitOutcome) Hermes() (
	publicExactFileReceipt,
	bool,
) {
	return outcome.hermes, outcome.hasHermes
}

func (outcome typedPublicInitOutcome) Overseer() (
	publicExactFileReceipt,
	bool,
) {
	return outcome.overseer, outcome.hasOverseer
}

func (outcome typedPublicInitOutcome) Kind() publicInitOutcomeKind {
	return outcome.kind
}

func (outcome typedPublicInitOutcome) Base() initexecution.InitExecutionOutcome {
	return outcome.base
}

func (outcome typedPublicInitOutcome) AgentSkills() (
	publicAgentSkillsReceipt,
	bool,
) {
	return outcome.agentSkills, outcome.hasAgentSkills
}

func (outcome typedPublicInitOutcome) DeprecatedSkillCleanup() (
	publicExactFileReceipt,
	bool,
) {
	present := outcome.hasCleanup && publicCleanupPlanHasKind(
		outcome.cleanupPlan,
		publicDeprecatedSkillTreeRemoval,
	)
	return publicCleanupReceiptForKind(
		outcome.cleanup,
		outcome.cleanupPlan,
		publicDeprecatedSkillTreeRemoval,
	), present
}

func (outcome typedPublicInitOutcome) LegacyCommandCleanup() (
	publicExactFileReceipt,
	bool,
) {
	present := outcome.hasCleanup && publicCleanupPlanHasKind(
		outcome.cleanupPlan,
		publicLegacyCommandFileRemoval,
	)
	return publicCleanupReceiptForKind(
		outcome.cleanup,
		outcome.cleanupPlan,
		publicLegacyCommandFileRemoval,
	), present
}

func (outcome typedPublicInitOutcome) LegacyCleanup() (
	publicExactFileReceipt,
	bool,
) {
	return outcome.cleanup, outcome.hasCleanup
}

func (outcome typedPublicInitOutcome) CoreEffects() (
	publicExactFileReceipt,
	bool,
) {
	return outcome.coreEffects, outcome.hasCoreEffects
}

type confirmedTypedPublicInitOperation struct {
	base           initexecution.ConfirmedInitOperation
	hermes         publicHermesPlan
	hasHermes      bool
	overseer       publicOverseerPlan
	hasOverseer    bool
	agentSkills    publicAgentSkillsPlan
	hasAgentSkills bool
	cleanup        publicDeprecatedSkillCleanupPlan
	hasCleanup     bool
	coordinator    initfs.PublicationCoordinator
	resources      []string
	confirmed      bool
}

func (operation confirmedTypedPublicInitOperation) Apply(
	ctx context.Context,
	executor typedPublicInitExecutor,
) (typedPublicInitOutcome, error) {
	if !operation.confirmed {
		return typedPublicInitOutcome{},
			fmt.Errorf("typed public init operation is not confirmed")
	}
	attempt, err := operation.coordinator.TryAcquire(
		operation.resources,
	)
	if err != nil {
		return typedPublicInitOutcome{
			kind: publicInitCoordinationFailure,
		}, err
	}
	if attempt.Kind() == initfs.PublicationCoordinationBusy {
		return typedPublicInitOutcome{
			kind: publicInitBusy,
		}, nil
	}
	lease, acquired := attempt.Lease()
	if !acquired {
		err := fmt.Errorf(
			"typed public init coordination returned no lease",
		)
		return typedPublicInitOutcome{
			kind: publicInitCoordinationFailure,
		}, err
	}
	outcome, applyErr := operation.applyUnderPublicationLease(
		ctx,
		executor,
		lease,
	)
	releaseErr := lease.Release()
	if releaseErr != nil && applyErr == nil {
		outcome.kind = publicInitPublicationIncomplete
	}
	return outcome, errors.Join(applyErr, releaseErr)
}

func (operation confirmedTypedPublicInitOperation) applyUnderPublicationLease(
	ctx context.Context,
	executor typedPublicInitExecutor,
	lease *initfs.PublicationCoordinationLease,
) (typedPublicInitOutcome, error) {
	base, err := operation.base.ApplyUnderPublicationLease(
		ctx,
		executor.base,
		lease,
	)
	if err != nil {
		outcome := typedPublicInitOutcome{
			kind: publicInitOutcomeKindFromBase(base.Kind()),
			base: base,
		}
		var coreFailure publicCoreApplicationError
		if errors.As(err, &coreFailure) {
			outcome.coreEffects = coreFailure.receipt
			outcome.hasCoreEffects = true
		}
		return outcome, err
	}
	if !publicBaseInitCompleted(base.Kind()) {
		return typedPublicInitOutcome{
			kind: publicInitOutcomeKindFromBase(base.Kind()),
			base: base,
		}, nil
	}
	var hermesReceipt publicExactFileReceipt
	if operation.hasHermes {
		hermesReceipt, err = applyPublicHermesPlan(
			ctx,
			operation.hermes,
		)
		if err != nil {
			return typedPublicInitOutcome{
				kind:      publicInitPublicationIncomplete,
				base:      base,
				hermes:    hermesReceipt,
				hasHermes: true,
			}, err
		}
	}
	var overseerReceipt publicExactFileReceipt
	if operation.hasOverseer {
		overseerReceipt, err = applyPublicOverseerPlan(
			ctx,
			operation.overseer,
		)
		if err != nil {
			return typedPublicInitOutcome{
				kind:        publicInitPublicationIncomplete,
				base:        base,
				hermes:      hermesReceipt,
				hasHermes:   operation.hasHermes,
				overseer:    overseerReceipt,
				hasOverseer: true,
			}, err
		}
	}
	var agentSkills publicAgentSkillsReceipt
	if operation.hasAgentSkills {
		agentSkills, err = applyPublicAgentSkillsPlan(
			ctx,
			operation.agentSkills,
		)
		if err != nil {
			return typedPublicInitOutcome{
				kind:           publicInitPublicationIncomplete,
				base:           base,
				hermes:         hermesReceipt,
				hasHermes:      operation.hasHermes,
				overseer:       overseerReceipt,
				hasOverseer:    operation.hasOverseer,
				agentSkills:    agentSkills,
				hasAgentSkills: true,
			}, err
		}
	}
	var cleanupReceipt publicExactFileReceipt
	if operation.hasCleanup {
		cleanupReceipt, err =
			applyPublicDeprecatedSkillCleanupPlan(
				ctx,
				operation.cleanup,
			)
		if err != nil {
			return typedPublicInitOutcome{
				kind:           publicInitPublicationIncomplete,
				base:           base,
				hermes:         hermesReceipt,
				hasHermes:      operation.hasHermes,
				overseer:       overseerReceipt,
				hasOverseer:    operation.hasOverseer,
				agentSkills:    agentSkills,
				hasAgentSkills: operation.hasAgentSkills,
				cleanup:        cleanupReceipt,
				cleanupPlan:    operation.cleanup,
				hasCleanup:     true,
			}, err
		}
	}
	kind := publicInitAlreadyCurrent
	changed := base.Kind() == initexecution.InitExecutionApplied
	changed = changed ||
		publicExactFileEffectsChange(operation.hermes.effects)
	changed = changed ||
		publicExactFileEffectsChange(operation.overseer.effects)
	changed = changed || agentSkills.ChangedPaths() > 0
	changed = changed || len(cleanupReceipt.Completed()) > 0
	if changed {
		kind = publicInitApplied
	}
	return typedPublicInitOutcome{
		kind:           kind,
		base:           base,
		hermes:         hermesReceipt,
		hasHermes:      operation.hasHermes,
		overseer:       overseerReceipt,
		hasOverseer:    operation.hasOverseer,
		agentSkills:    agentSkills,
		hasAgentSkills: operation.hasAgentSkills,
		cleanup:        cleanupReceipt,
		cleanupPlan:    operation.cleanup,
		hasCleanup:     operation.hasCleanup,
	}, nil
}

func publicInitOutcomeKindFromBase(
	kind initexecution.InitExecutionOutcomeKind,
) publicInitOutcomeKind {
	switch kind {
	case initexecution.InitExecutionApplied:
		return publicInitApplied
	case initexecution.InitExecutionAlreadyCurrent:
		return publicInitAlreadyCurrent
	case initexecution.InitExecutionPlanBlocked:
		return publicInitPlanBlocked
	case initexecution.InitExecutionBusy:
		return publicInitBusy
	case initexecution.InitExecutionCoordinationUnavailable:
		return publicInitCoordinationFailure
	case initexecution.InitExecutionCoreUnconfirmed:
		return publicInitCoreUnconfirmed
	case initexecution.InitExecutionHostIncomplete:
		return publicInitPublicationIncomplete
	default:
		return publicInitCoreUnconfirmed
	}
}

func publicExactFileEffectsChange(
	effects []publicExactFileEffect,
) bool {
	for _, effect := range effects {
		if effect.kind != publicExactFilePreserve {
			return true
		}
	}
	return false
}

func publicBaseInitCompleted(
	kind initexecution.InitExecutionOutcomeKind,
) bool {
	return slices.Contains(
		[]initexecution.InitExecutionOutcomeKind{
			initexecution.InitExecutionApplied,
			initexecution.InitExecutionAlreadyCurrent,
		},
		kind,
	)
}
