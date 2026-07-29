// Package initexecution coordinates initialization effects after an exact
// initplanning.InitPlan has been compiled.
package initexecution

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectledgermigration"
)

type CoreEffectOutcomeKind string

const (
	CoreEffectApplied        CoreEffectOutcomeKind = "applied"
	CoreEffectAlreadyCurrent CoreEffectOutcomeKind = "already_current"
)

type CoreEffectReceipt struct {
	outcome      CoreEffectOutcomeKind
	effect       initplanning.CoreEffectKind
	projectRoot  string
	projectID    string
	databasePath string
	beforeSchema int
	afterSchema  int
}

func NewCoreEffectReceipt(
	outcome CoreEffectOutcomeKind,
	effect initplanning.CoreEffectKind,
	projectRoot string,
	projectID string,
	databasePath string,
	beforeSchema int,
	afterSchema int,
) (CoreEffectReceipt, error) {
	if err := validateCoreEffectReceipt(
		outcome,
		effect,
		projectRoot,
		projectID,
		databasePath,
		beforeSchema,
		afterSchema,
	); err != nil {
		return CoreEffectReceipt{}, err
	}
	return CoreEffectReceipt{
		outcome:      outcome,
		effect:       effect,
		projectRoot:  projectRoot,
		projectID:    projectID,
		databasePath: databasePath,
		beforeSchema: beforeSchema,
		afterSchema:  afterSchema,
	}, nil
}

func validateCoreEffectReceipt(
	outcome CoreEffectOutcomeKind,
	effect initplanning.CoreEffectKind,
	projectRoot string,
	projectID string,
	databasePath string,
	beforeSchema int,
	afterSchema int,
) error {
	if !canonicalNonRootPath(projectRoot) ||
		!canonicalNonRootPath(databasePath) {
		return fmt.Errorf("core effect receipt path is invalid")
	}
	if _, err := projectidentity.ParseProjectID(projectID); err != nil {
		return fmt.Errorf("core effect receipt project identity: %w", err)
	}
	switch outcome {
	case CoreEffectApplied:
		if validAppliedCoreTransition(effect, beforeSchema, afterSchema) {
			return nil
		}
	case CoreEffectAlreadyCurrent:
		if effect == initplanning.CoreVerifyCurrent &&
			beforeSchema > 0 &&
			beforeSchema == afterSchema {
			return nil
		}
	}
	return fmt.Errorf(
		"core effect receipt transition is invalid: %s %s %d -> %d",
		outcome,
		effect,
		beforeSchema,
		afterSchema,
	)
}

func validAppliedCoreTransition(
	effect initplanning.CoreEffectKind,
	beforeSchema int,
	afterSchema int,
) bool {
	switch effect {
	case initplanning.CoreInitialize:
		return beforeSchema == 0 && afterSchema > 0
	case initplanning.CoreMigrate:
		return beforeSchema > 0 && afterSchema > beforeSchema
	default:
		return false
	}
}

func canonicalNonRootPath(path string) bool {
	if path == "" ||
		!filepath.IsAbs(path) ||
		filepath.Clean(path) != path {
		return false
	}
	volumeRoot := filepath.VolumeName(path) + string(filepath.Separator)
	return path != volumeRoot
}

func (receipt CoreEffectReceipt) Outcome() CoreEffectOutcomeKind {
	return receipt.outcome
}

func (receipt CoreEffectReceipt) Effect() initplanning.CoreEffectKind {
	return receipt.effect
}

func (receipt CoreEffectReceipt) ProjectRoot() string {
	return receipt.projectRoot
}

func (receipt CoreEffectReceipt) ProjectID() string {
	return receipt.projectID
}

func (receipt CoreEffectReceipt) DatabasePath() string {
	return receipt.databasePath
}

func (receipt CoreEffectReceipt) BeforeSchema() int {
	return receipt.beforeSchema
}

func (receipt CoreEffectReceipt) AfterSchema() int {
	return receipt.afterSchema
}

type CoreEffectPort interface {
	ApplyCore(
		context.Context,
		initplanning.CoreProjectPlan,
	) (CoreEffectReceipt, error)
}

type CoreEffectFunc func(
	context.Context,
	initplanning.CoreProjectPlan,
) (CoreEffectReceipt, error)

func (apply CoreEffectFunc) ApplyCore(
	ctx context.Context,
	plan initplanning.CoreProjectPlan,
) (CoreEffectReceipt, error) {
	return apply(ctx, plan)
}

type ExistingProjectCoreEffect struct{}

func (ExistingProjectCoreEffect) ApplyCore(
	ctx context.Context,
	plan initplanning.CoreProjectPlan,
) (CoreEffectReceipt, error) {
	request, err := projectledgermigration.NewRequest(
		plan.ProjectRoot(),
		plan.ProjectID().String(),
	)
	if err != nil {
		return CoreEffectReceipt{}, err
	}
	switch plan.Effect() {
	case initplanning.CoreMigrate:
		return applyExistingProjectMigration(ctx, plan, request)
	case initplanning.CoreVerifyCurrent:
		return verifyExistingProjectCore(ctx, plan, request)
	default:
		return CoreEffectReceipt{}, fmt.Errorf(
			"existing-project exact core effect cannot apply %s",
			plan.Effect(),
		)
	}
}

func applyExistingProjectMigration(
	ctx context.Context,
	plan initplanning.CoreProjectPlan,
	request projectledgermigration.Request,
) (CoreEffectReceipt, error) {
	transition, err := projectledgermigration.NewExactTransition(
		plan.BeforeSchema(),
		plan.AfterSchema(),
	)
	if err != nil {
		return CoreEffectReceipt{}, err
	}
	result, err := projectledgermigration.ApplyExact(
		ctx,
		request,
		transition,
	)
	if err != nil {
		return CoreEffectReceipt{}, err
	}
	outcome, effect, err := projectMigrationReceiptPosture(result)
	if err != nil {
		return CoreEffectReceipt{}, err
	}
	return NewCoreEffectReceipt(
		outcome,
		effect,
		result.ProjectRoot,
		result.ProjectID,
		result.DatabasePath,
		result.BeforeSchema,
		result.AfterSchema,
	)
}

func verifyExistingProjectCore(
	ctx context.Context,
	plan initplanning.CoreProjectPlan,
	request projectledgermigration.Request,
) (CoreEffectReceipt, error) {
	observation, err := projectledgermigration.Observe(ctx, request)
	if err != nil {
		return CoreEffectReceipt{}, err
	}
	exact := observation.ProjectRoot == plan.ProjectRoot()
	exact = exact && observation.ProjectID == plan.ProjectID().String()
	exact = exact && observation.DatabasePath == plan.DatabasePath()
	exact = exact && observation.ObservedSchema == plan.BeforeSchema()
	exact = exact && observation.CompiledSchema == plan.AfterSchema()
	if !exact {
		return CoreEffectReceipt{}, fmt.Errorf(
			"existing project core differs from exact verification plan",
		)
	}
	return NewCoreEffectReceipt(
		CoreEffectAlreadyCurrent,
		initplanning.CoreVerifyCurrent,
		observation.ProjectRoot,
		observation.ProjectID,
		observation.DatabasePath,
		observation.ObservedSchema,
		observation.CompiledSchema,
	)
}

func projectMigrationReceiptPosture(
	result projectledgermigration.Result,
) (CoreEffectOutcomeKind, initplanning.CoreEffectKind, error) {
	switch result.Outcome {
	case projectledgermigration.OutcomeApplied:
		return CoreEffectApplied, initplanning.CoreMigrate, nil
	case projectledgermigration.OutcomeAlreadyCurrent:
		return CoreEffectAlreadyCurrent, initplanning.CoreVerifyCurrent, nil
	default:
		return "", "", fmt.Errorf(
			"project migration returned an unknown outcome %s",
			result.Outcome,
		)
	}
}

type HostPublicationPort interface {
	Publish(
		initplanning.HostPublicationBatch,
		initfs.ManifestStore,
	) (initfs.HostPublicationOutcome, error)
}

type HostPublicationFunc func(
	initplanning.HostPublicationBatch,
	initfs.ManifestStore,
) (initfs.HostPublicationOutcome, error)

func (publish HostPublicationFunc) Publish(
	batch initplanning.HostPublicationBatch,
	store initfs.ManifestStore,
) (initfs.HostPublicationOutcome, error) {
	return publish(batch, store)
}

type HostManifestBinding struct {
	binding initplanning.HostBindingID
	store   initfs.ManifestStore
}

func NewHostManifestBinding(
	binding initplanning.HostBindingID,
	store initfs.ManifestStore,
) (HostManifestBinding, error) {
	exact, err := initplanning.NewHostBindingID(
		binding.Host(),
		binding.Scope(),
	)
	if err != nil {
		return HostManifestBinding{}, err
	}
	if exact != binding {
		return HostManifestBinding{}, fmt.Errorf(
			"host manifest binding identity is not canonical",
		)
	}
	if !canonicalNonRootPath(store.Root()) ||
		!canonicalNonRootPath(store.Path()) ||
		store.Path() == store.Root() {
		return HostManifestBinding{}, fmt.Errorf(
			"host %s manifest store is invalid",
			binding.String(),
		)
	}
	return HostManifestBinding{
		binding: binding,
		store:   store,
	}, nil
}

type HostManifestRegistry struct {
	stores map[initplanning.HostBindingID]initfs.ManifestStore
}

func NewHostManifestRegistry(
	bindings []HostManifestBinding,
) (HostManifestRegistry, error) {
	stores := make(
		map[initplanning.HostBindingID]initfs.ManifestStore,
		len(bindings),
	)
	paths := make(
		map[string]initplanning.HostBindingID,
		len(bindings),
	)
	for _, binding := range bindings {
		if binding.binding.String() == "/" ||
			binding.store.Path() == "" {
			return HostManifestRegistry{}, fmt.Errorf(
				"host manifest registry contains an invalid binding",
			)
		}
		if _, duplicate := stores[binding.binding]; duplicate {
			return HostManifestRegistry{}, fmt.Errorf(
				"host manifest registry repeats host binding %s",
				binding.binding.String(),
			)
		}
		if other, duplicate := paths[binding.store.Path()]; duplicate {
			return HostManifestRegistry{}, fmt.Errorf(
				"hosts %s and %s share manifest path %s",
				other.String(),
				binding.binding.String(),
				binding.store.Path(),
			)
		}
		stores[binding.binding] = binding.store
		paths[binding.store.Path()] = binding.binding
	}
	return HostManifestRegistry{stores: stores}, nil
}

type preparedHostPublication struct {
	binding initplanning.HostBindingID
	batch   initplanning.HostPublicationBatch
	store   initfs.ManifestStore
}

type InitExecutionOutcomeKind string

const (
	InitExecutionApplied                 InitExecutionOutcomeKind = "applied"
	InitExecutionAlreadyCurrent          InitExecutionOutcomeKind = "already_current"
	InitExecutionPlanBlocked             InitExecutionOutcomeKind = "plan_blocked"
	InitExecutionBusy                    InitExecutionOutcomeKind = "busy"
	InitExecutionCoordinationUnavailable InitExecutionOutcomeKind = "coordination_unavailable"
	InitExecutionCoreUnconfirmed         InitExecutionOutcomeKind = "core_effect_unconfirmed"
	InitExecutionHostIncomplete          InitExecutionOutcomeKind = "core_applied_host_incomplete"
)

type HostExecutionReceipt struct {
	binding     initplanning.HostBindingID
	batchDigest string
	outcome     initfs.HostPublicationOutcome
}

func (receipt HostExecutionReceipt) Host() initplanning.HostID {
	return receipt.binding.Host()
}

func (receipt HostExecutionReceipt) Scope() initplanning.InstallScope {
	return receipt.binding.Scope()
}

func (receipt HostExecutionReceipt) BindingID() initplanning.HostBindingID {
	return receipt.binding
}

func (receipt HostExecutionReceipt) BatchDigest() string {
	return receipt.batchDigest
}

func (receipt HostExecutionReceipt) Outcome() initfs.HostPublicationOutcome {
	return receipt.outcome
}

type InitExecutionOutcome struct {
	kind             InitExecutionOutcomeKind
	core             CoreEffectReceipt
	hasCore          bool
	hosts            []HostExecutionReceipt
	pendingBindings  []initplanning.HostBindingID
	reason           string
	coordinationPath string
	resourceDigest   string
}

func (outcome InitExecutionOutcome) Kind() InitExecutionOutcomeKind {
	return outcome.kind
}

func (outcome InitExecutionOutcome) CoreReceipt() (CoreEffectReceipt, bool) {
	return outcome.core, outcome.hasCore
}

func (outcome InitExecutionOutcome) HostReceipts() []HostExecutionReceipt {
	return slices.Clone(outcome.hosts)
}

func (outcome InitExecutionOutcome) PendingHosts() []initplanning.HostID {
	hosts := make(
		[]initplanning.HostID,
		len(outcome.pendingBindings),
	)
	for index, binding := range outcome.pendingBindings {
		hosts[index] = binding.Host()
	}
	return hosts
}

func (outcome InitExecutionOutcome) PendingBindings() []initplanning.HostBindingID {
	return slices.Clone(outcome.pendingBindings)
}

func (outcome InitExecutionOutcome) Reason() string {
	return outcome.reason
}

func (outcome InitExecutionOutcome) CoordinationPath() string {
	return outcome.coordinationPath
}

func (outcome InitExecutionOutcome) ResourceDigest() string {
	return outcome.resourceDigest
}

func (outcome InitExecutionOutcome) PartialEffectBoundary() string {
	if outcome.kind != InitExecutionHostIncomplete {
		return ""
	}
	return string(InitExecutionHostIncomplete)
}

type Executor struct {
	core CoreEffectPort
	host HostPublicationPort
}

func NewExecutor(
	core CoreEffectPort,
	host HostPublicationPort,
) (Executor, error) {
	if core == nil || host == nil {
		return Executor{}, fmt.Errorf(
			"init executor requires core and host effect ports",
		)
	}
	return Executor{core: core, host: host}, nil
}

func (executor Executor) Execute(
	ctx context.Context,
	plan initplanning.InitPlan,
	registry HostManifestRegistry,
	coordinator initfs.PublicationCoordinator,
) (InitExecutionOutcome, error) {
	if ctx == nil {
		return InitExecutionOutcome{}, fmt.Errorf(
			"init execution context is required",
		)
	}
	if executor.core == nil || executor.host == nil {
		return InitExecutionOutcome{}, fmt.Errorf("init executor is invalid")
	}
	if plan.Readiness() == initplanning.PlanBlocked {
		return InitExecutionOutcome{
			kind:            InitExecutionPlanBlocked,
			pendingBindings: selectedHostBindings(plan),
			reason:          "compiled_init_plan_has_preserved_conflicts",
		}, nil
	}
	prepared, err := prepareHostPublications(plan, registry)
	if err != nil {
		return InitExecutionOutcome{}, err
	}
	if len(prepared) == 0 {
		return executor.executePrepared(ctx, plan, prepared)
	}
	if err := validateCoordinatorPlacement(coordinator, prepared); err != nil {
		return InitExecutionOutcome{}, err
	}
	attempt, err := coordinator.TryAcquire(
		executionResources(plan, registry),
	)
	if err != nil {
		return InitExecutionOutcome{
			kind:            InitExecutionCoordinationUnavailable,
			pendingBindings: preparedHostBindings(prepared),
			reason:          err.Error(),
		}, err
	}
	if attempt.Kind() == initfs.PublicationCoordinationBusy {
		return InitExecutionOutcome{
			kind:             InitExecutionBusy,
			pendingBindings:  preparedHostBindings(prepared),
			reason:           "publication_coordination_busy",
			coordinationPath: attempt.LockPath(),
			resourceDigest:   attempt.ResourceDigest(),
		}, nil
	}
	lease, acquired := attempt.Lease()
	if !acquired {
		err := fmt.Errorf("publication coordination returned no acquired lease")
		return InitExecutionOutcome{
			kind:             InitExecutionCoordinationUnavailable,
			pendingBindings:  preparedHostBindings(prepared),
			reason:           err.Error(),
			coordinationPath: attempt.LockPath(),
			resourceDigest:   attempt.ResourceDigest(),
		}, err
	}
	outcome, executeErr := executor.executePrepared(
		ctx,
		plan,
		prepared,
	)
	outcome.coordinationPath = attempt.LockPath()
	outcome.resourceDigest = attempt.ResourceDigest()
	releaseErr := lease.Release()
	return outcome, errors.Join(executeErr, releaseErr)
}

func (executor Executor) ExecuteUnderPublicationLease(
	ctx context.Context,
	plan initplanning.InitPlan,
	registry HostManifestRegistry,
) (InitExecutionOutcome, error) {
	if ctx == nil {
		return InitExecutionOutcome{}, fmt.Errorf(
			"init execution context is required",
		)
	}
	if executor.core == nil || executor.host == nil {
		return InitExecutionOutcome{}, fmt.Errorf(
			"init executor is invalid",
		)
	}
	if plan.Readiness() == initplanning.PlanBlocked {
		return InitExecutionOutcome{
			kind:            InitExecutionPlanBlocked,
			pendingBindings: selectedHostBindings(plan),
			reason:          "compiled_init_plan_has_preserved_conflicts",
		}, nil
	}
	prepared, err := prepareHostPublications(plan, registry)
	if err != nil {
		return InitExecutionOutcome{}, err
	}
	return executor.executePrepared(ctx, plan, prepared)
}

func (executor Executor) executePrepared(
	ctx context.Context,
	plan initplanning.InitPlan,
	prepared []preparedHostPublication,
) (InitExecutionOutcome, error) {
	coreReceipt, err := executor.core.ApplyCore(ctx, plan.Core())
	if err != nil {
		return InitExecutionOutcome{
			kind:            InitExecutionCoreUnconfirmed,
			pendingBindings: preparedHostBindings(prepared),
			reason:          err.Error(),
		}, err
	}
	if err := requireExactCoreReceipt(plan.Core(), coreReceipt); err != nil {
		return InitExecutionOutcome{
			kind:            InitExecutionCoreUnconfirmed,
			core:            coreReceipt,
			hasCore:         true,
			pendingBindings: preparedHostBindings(prepared),
			reason:          err.Error(),
		}, err
	}
	return executor.publishHosts(coreReceipt, prepared)
}

func validateCoordinatorPlacement(
	coordinator initfs.PublicationCoordinator,
	prepared []preparedHostPublication,
) error {
	if !canonicalNonRootPath(coordinator.Root()) ||
		!canonicalNonRootPath(coordinator.LockPath()) {
		return fmt.Errorf("publication coordinator is invalid")
	}
	for _, publication := range prepared {
		if coordinator.LockPath() == publication.store.Path() {
			return fmt.Errorf(
				"publication coordination lock collides with host %s manifest",
				publication.binding.String(),
			)
		}
		for _, root := range publication.batch.TargetRoots() {
			if pathInsideRoot(coordinator.LockPath(), root) {
				return fmt.Errorf(
					"publication coordination lock is inside host %s target root",
					publication.binding.String(),
				)
			}
		}
	}
	return nil
}

func pathInsideRoot(
	path string,
	root string,
) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." ||
		(relative != ".." &&
			!strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func executionResources(
	plan initplanning.InitPlan,
	registry HostManifestRegistry,
) []string {
	resources := []string{
		plan.Core().ProjectRoot(),
		plan.Core().DatabasePath(),
	}
	for _, host := range plan.Hosts() {
		resources = append(resources, host.TargetRoots()...)
	}
	for _, store := range registry.stores {
		resources = append(
			resources,
			store.Root(),
			store.Path(),
		)
	}
	return resources
}

func prepareHostPublications(
	plan initplanning.InitPlan,
	registry HostManifestRegistry,
) ([]preparedHostPublication, error) {
	hostPlans := plan.Hosts()
	if len(hostPlans) != len(registry.stores) {
		return nil, fmt.Errorf(
			"host manifest registry has %d bindings for %d selected hosts",
			len(registry.stores),
			len(hostPlans),
		)
	}
	prepared := make([]preparedHostPublication, 0, len(hostPlans))
	stepPaths := make(
		map[string]initplanning.HostBindingID,
	)
	manifestPaths := make(
		map[string]initplanning.HostBindingID,
		len(registry.stores),
	)
	for binding, store := range registry.stores {
		manifestPaths[store.Path()] = binding
	}
	for _, hostPlan := range hostPlans {
		binding := hostPlan.BindingID()
		store, exists := registry.stores[binding]
		if !exists {
			return nil, fmt.Errorf(
				"selected host binding %s has no manifest store",
				binding.String(),
			)
		}
		batch, err := initplanning.BuildHostPublicationBatch(hostPlan)
		if err != nil {
			return nil, err
		}
		if err := rejectManifestStepCollision(
			binding,
			store.Path(),
			batch,
			stepPaths,
			manifestPaths,
		); err != nil {
			return nil, err
		}
		prepared = append(prepared, preparedHostPublication{
			binding: binding,
			batch:   batch,
			store:   store,
		})
	}
	return prepared, nil
}

func rejectManifestStepCollision(
	binding initplanning.HostBindingID,
	manifestPath string,
	batch initplanning.HostPublicationBatch,
	stepPaths map[string]initplanning.HostBindingID,
	manifestPaths map[string]initplanning.HostBindingID,
) error {
	if owner, collision := stepPaths[manifestPath]; collision {
		return fmt.Errorf(
			"host %s manifest path collides with host %s carrier path %s",
			binding.String(),
			owner.String(),
			manifestPath,
		)
	}
	for _, step := range batch.Steps() {
		if owner, collision := manifestPaths[step.Path()]; collision {
			return fmt.Errorf(
				"host %s carrier path collides with host %s manifest path %s",
				binding.String(),
				owner.String(),
				step.Path(),
			)
		}
		if owner, collision := stepPaths[step.Path()]; collision {
			return fmt.Errorf(
				"hosts %s and %s share carrier path %s",
				owner.String(),
				binding.String(),
				step.Path(),
			)
		}
		stepPaths[step.Path()] = binding
	}
	return nil
}

func requireExactCoreReceipt(
	plan initplanning.CoreProjectPlan,
	receipt CoreEffectReceipt,
) error {
	if receipt.projectRoot != plan.ProjectRoot() ||
		receipt.projectID != plan.ProjectID().String() ||
		receipt.databasePath != plan.DatabasePath() ||
		receipt.effect != plan.Effect() ||
		receipt.beforeSchema != plan.BeforeSchema() ||
		receipt.afterSchema != plan.AfterSchema() {
		return fmt.Errorf(
			"core effect receipt differs from the compiled plan",
		)
	}
	return nil
}

func (executor Executor) publishHosts(
	coreReceipt CoreEffectReceipt,
	prepared []preparedHostPublication,
) (InitExecutionOutcome, error) {
	receipts := make([]HostExecutionReceipt, 0, len(prepared))
	for index, publication := range prepared {
		outcome, err := executor.host.Publish(
			publication.batch,
			publication.store,
		)
		receipts = append(receipts, HostExecutionReceipt{
			binding:     publication.binding,
			batchDigest: publication.batch.Digest(),
			outcome:     outcome,
		})
		if err != nil {
			return incompleteHostExecution(
				coreReceipt,
				receipts,
				prepared[index+1:],
				err.Error(),
			), err
		}
		if !hostPublicationComplete(outcome.Kind()) {
			return incompleteHostExecution(
				coreReceipt,
				receipts,
				prepared[index+1:],
				"host_publication_"+string(outcome.Kind()),
			), nil
		}
	}
	kind := InitExecutionApplied
	if coreReceipt.outcome == CoreEffectAlreadyCurrent &&
		allHostPublicationsCurrent(receipts) {
		kind = InitExecutionAlreadyCurrent
	}
	return InitExecutionOutcome{
		kind:    kind,
		core:    coreReceipt,
		hasCore: true,
		hosts:   slices.Clone(receipts),
	}, nil
}

func hostPublicationComplete(
	kind initfs.HostPublicationOutcomeKind,
) bool {
	return kind == initfs.HostPublicationApplied ||
		kind == initfs.HostPublicationAlreadyCurrent
}

func allHostPublicationsCurrent(
	receipts []HostExecutionReceipt,
) bool {
	for _, receipt := range receipts {
		if receipt.outcome.Kind() != initfs.HostPublicationAlreadyCurrent {
			return false
		}
	}
	return true
}

func incompleteHostExecution(
	coreReceipt CoreEffectReceipt,
	receipts []HostExecutionReceipt,
	remaining []preparedHostPublication,
	reason string,
) InitExecutionOutcome {
	pendingBindings := preparedHostBindings(remaining)
	if len(receipts) > 0 {
		last := receipts[len(receipts)-1]
		if !hostPublicationComplete(last.outcome.Kind()) {
			pendingBindings = append(
				[]initplanning.HostBindingID{last.binding},
				pendingBindings...,
			)
		}
	}
	return InitExecutionOutcome{
		kind:            InitExecutionHostIncomplete,
		core:            coreReceipt,
		hasCore:         true,
		hosts:           slices.Clone(receipts),
		pendingBindings: pendingBindings,
		reason:          reason,
	}
}

func selectedHostBindings(
	plan initplanning.InitPlan,
) []initplanning.HostBindingID {
	hosts := plan.Hosts()
	result := make([]initplanning.HostBindingID, len(hosts))
	for index, host := range hosts {
		result[index] = host.BindingID()
	}
	return result
}

func preparedHostBindings(
	prepared []preparedHostPublication,
) []initplanning.HostBindingID {
	result := make([]initplanning.HostBindingID, len(prepared))
	for index, publication := range prepared {
		result[index] = publication.binding
	}
	return result
}
