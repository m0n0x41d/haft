package initplanning

import (
	"fmt"
	"slices"
	"sort"
)

type PlanReadiness string

const (
	PlanReady   PlanReadiness = "ready"
	PlanBlocked PlanReadiness = "blocked"
)

type InitPlan struct {
	intent    InitIntent
	core      CoreProjectPlan
	hosts     []HostAdapterInstallPlan
	readiness PlanReadiness
}

func CompileInitPlan(
	intent InitIntent,
	core CoreProjectPlan,
	adapterPlans []HostAdapterInstallPlan,
	catalog AdapterCatalog,
) (InitPlan, error) {
	if intent.policy == "" || intent.projectRoot == "" || intent.projectID.String() == "" {
		return InitPlan{}, fmt.Errorf("initialization intent is invalid")
	}
	if core.projectRoot != intent.projectRoot || core.projectID != intent.projectID {
		return InitPlan{}, fmt.Errorf("core plan belongs to another project")
	}
	if !core.basis.valid() {
		return InitPlan{}, fmt.Errorf("core plan has invalid TypeEnv readiness")
	}
	plansByBinding := make(
		map[HostBindingID]HostAdapterInstallPlan,
		len(adapterPlans),
	)
	for _, adapterPlan := range adapterPlans {
		binding := adapterPlan.BindingID()
		if _, duplicate := plansByBinding[binding]; duplicate {
			return InitPlan{}, fmt.Errorf(
				"adapter plan repeats host binding %s",
				binding.String(),
			)
		}
		plansByBinding[binding] = adapterPlan
	}
	selected := intent.hosts.Values()
	if len(plansByBinding) != len(selected) {
		return InitPlan{}, fmt.Errorf("selected host set and adapter plan set differ")
	}
	ordered := make([]HostAdapterInstallPlan, 0, len(selected))
	for _, selection := range selected {
		if err := catalog.validate(selection); err != nil {
			return InitPlan{}, err
		}
		binding := selection.BindingID()
		edition, err := catalog.edition(binding)
		if err != nil {
			return InitPlan{}, err
		}
		adapterPlan, ok := plansByBinding[binding]
		if !ok {
			return InitPlan{}, fmt.Errorf(
				"selected host binding %s has no adapter plan",
				binding.String(),
			)
		}
		if err := validateAdapterPlanMatch(
			intent,
			selection,
			edition,
			adapterPlan,
		); err != nil {
			return InitPlan{}, err
		}
		ordered = append(ordered, cloneHostAdapterPlan(adapterPlan))
	}
	if err := rejectCrossAdapterPathOverlap(ordered); err != nil {
		return InitPlan{}, err
	}
	readiness := PlanReady
	for _, adapterPlan := range ordered {
		if len(adapterPlan.conflicts) > 0 ||
			len(adapterPlan.ManagedFragmentConflicts()) > 0 {
			readiness = PlanBlocked
		}
	}
	return InitPlan{
		intent:    cloneInitIntent(intent),
		core:      core,
		hosts:     ordered,
		readiness: readiness,
	}, nil
}

func validateAdapterPlanMatch(
	intent InitIntent,
	selection HostSelection,
	edition string,
	plan HostAdapterInstallPlan,
) error {
	if plan.host != selection.host ||
		plan.scope != selection.scope ||
		!plan.components.equal(selection.components) {
		return fmt.Errorf("host %s adapter plan differs from selected intent", selection.host)
	}
	if plan.edition != edition {
		return fmt.Errorf(
			"host %s adapter edition %s differs from catalog edition %s",
			selection.host,
			plan.edition,
			edition,
		)
	}
	if plan.projectRoot != intent.projectRoot || plan.projectID != intent.projectID {
		return fmt.Errorf("host %s adapter plan belongs to another project", selection.host)
	}
	for index, expectation := range plan.expectations {
		output := plan.outputs[index]
		if expectation.kind == PredecessorCurrentOwned && expectation.digest != output.digest {
			return fmt.Errorf(
				"host %s path %s claims current ownership over different bytes",
				selection.host,
				output.path,
			)
		}
		if expectation.kind == PredecessorOutdatedOwned && expectation.digest == output.digest {
			return fmt.Errorf(
				"host %s path %s claims outdated ownership over current bytes",
				selection.host,
				output.path,
			)
		}
	}
	return nil
}

func rejectCrossAdapterPathOverlap(plans []HostAdapterInstallPlan) error {
	owners := make(map[string]HostBindingID)
	for _, plan := range plans {
		binding := plan.BindingID()
		for _, output := range plan.outputs {
			if owner, duplicate := owners[output.path]; duplicate {
				return fmt.Errorf(
					"host adapters %s and %s both plan path %s",
					owner.String(),
					binding.String(),
					output.path,
				)
			}
			owners[output.path] = binding
		}
		for _, removal := range plan.removals {
			path := removal.expectation.path
			if owner, duplicate := owners[path]; duplicate {
				return fmt.Errorf(
					"host adapters %s and %s both plan path %s",
					owner.String(),
					binding.String(),
					path,
				)
			}
			owners[path] = binding
		}
		for _, carrier := range plan.managedCarriers {
			path := carrier.Path()
			if owner, duplicate := owners[path]; duplicate {
				return fmt.Errorf(
					"host adapters %s and %s both plan shared carrier %s",
					owner.String(),
					binding.String(),
					path,
				)
			}
			owners[path] = binding
		}
	}
	return nil
}

func cloneInitIntent(intent InitIntent) InitIntent {
	return InitIntent{
		policy:      intent.policy,
		projectRoot: intent.projectRoot,
		projectID:   intent.projectID,
		hosts:       SelectedHostSet{values: intent.hosts.Values()},
	}
}

func cloneHostAdapterPlan(plan HostAdapterInstallPlan) HostAdapterInstallPlan {
	return HostAdapterInstallPlan{
		host:             plan.host,
		edition:          plan.edition,
		publication:      plan.publication,
		projectRoot:      plan.projectRoot,
		projectID:        plan.projectID,
		scope:            plan.scope,
		components:       ComponentSet{values: plan.components.Values()},
		targetRoots:      slices.Clone(plan.targetRoots),
		expectations:     slices.Clone(plan.expectations),
		outputs:          cloneRenderedOutputs(plan.outputs),
		removals:         slices.Clone(plan.removals),
		conflicts:        slices.Clone(plan.conflicts),
		managedFragments: cloneManagedFragments(plan.managedFragments),
		managedCarriers:  cloneManagedCarrierInstallPlans(plan.managedCarriers),
		manifestBasis:    plan.manifestBasis,
		recovery:         RecoveryOperation{argv: plan.recovery.Argv()},
	}
}

func (plan InitPlan) Intent() InitIntent {
	return cloneInitIntent(plan.intent)
}

func (plan InitPlan) Core() CoreProjectPlan {
	return plan.core
}

func (plan InitPlan) Hosts() []HostAdapterInstallPlan {
	result := make([]HostAdapterInstallPlan, len(plan.hosts))
	for index, host := range plan.hosts {
		result[index] = cloneHostAdapterPlan(host)
	}
	return result
}

func (plan InitPlan) Readiness() PlanReadiness {
	return plan.readiness
}

type FileEffectKind string

const (
	FileCreate            FileEffectKind = "create"
	FilePreserve          FileEffectKind = "preserve"
	FileUpdate            FileEffectKind = "update"
	FileAdoptLegacy       FileEffectKind = "adopt_known_legacy"
	FileUpdateLegacy      FileEffectKind = "update_known_legacy"
	FileRemoveOwnedOrphan FileEffectKind = "remove_owned_orphan"
	FileConflict          FileEffectKind = "conflict"
)

type FileEffectPreview struct {
	Path               string
	Component          Component
	Components         []Component
	Effect             FileEffectKind
	PredecessorKind    PredecessorKind
	PredecessorDigest  string
	PredecessorMode    uint32
	ManifestPathDigest string
	ManifestPathMode   uint32
	OwnershipKind      OwnershipBasisKind
	OwnershipRef       string
	OwnershipDigest    string
	RenderedDigest     string
	RenderedMode       uint32
	ConflictKind       ConflictKind
	Reason             string
	SharedCarrier      bool
}

type ManagedFragmentEffectPreview struct {
	CarrierPath     string
	Component       Component
	Kind            ManagedFragmentKind
	Selector        string
	MemberID        string
	MergeEdition    string
	Effect          ManagedFragmentEffectKind
	ExpectedKind    ManagedFragmentObservationKind
	ExpectedDigest  string
	DesiredDigest   string
	ConflictKind    ManagedFragmentConflictKind
	Reason          string
	OwnershipKind   OwnershipBasisKind
	OwnershipRef    string
	OwnershipDigest string
}

type HostPlanPreview struct {
	Host                HostID
	Edition             string
	Scope               InstallScope
	Components          []Component
	TargetRoots         []string
	HaftVersion         string
	ExecutablePath      string
	ExecutableDigest    string
	SkillBundleDigest   string
	KernelCatalogDigest string
	Effects             []FileEffectPreview
	ManagedFragments    []ManagedFragmentEffectPreview
	RecoveryArgv        []string
}

type CorePlanPreview struct {
	ProjectRoot              string
	ProjectID                string
	DatabasePath             string
	DatabaseSeedKind         CoreDatabaseSeedKind
	DatabaseSeedObservedPath string
	DatabaseSeedPath         string
	DatabaseSeedHash         string
	RootMigrationKind        CoreRootMigrationKind
	RootMigrationSource      string
	RootMigrationTarget      string
	Effect                   CoreEffectKind
	BeforeSchema             int
	AfterSchema              int
	BasisKind                BasisReadinessKind
	BasisReason              string
	HeadRef                  string
	HeadRevision             int64
	SelectedComposite        string
	FileEffects              []CoreFileEffectPreview
}

type CoreFileEffectPreview struct {
	Path           string
	Kind           CoreFileEffectKind
	ExpectedDigest string
	ExpectedMode   uint32
	RenderedDigest string
	RenderedMode   uint32
}

type InitPlanPreview struct {
	InvocationPolicy      InvocationPolicy
	Readiness             PlanReadiness
	Core                  CorePlanPreview
	Hosts                 []HostPlanPreview
	ApplyOrder            []string
	PartialEffectBoundary string
}

func (plan InitPlan) Preview() InitPlanPreview {
	hosts := make([]HostPlanPreview, len(plan.hosts))
	for index, host := range plan.hosts {
		hosts[index] = previewHostPlan(host)
	}
	basis := plan.core.basis
	coreFiles := make(
		[]CoreFileEffectPreview,
		len(plan.core.files),
	)
	for index, effect := range plan.core.files {
		coreFiles[index] = CoreFileEffectPreview{
			Path:           effect.path,
			Kind:           effect.kind,
			ExpectedDigest: effect.expectedDigest,
			ExpectedMode:   uint32(effect.expectedMode.Perm()),
			RenderedDigest: effect.renderedDigest,
			RenderedMode:   uint32(effect.mode.Perm()),
		}
	}
	return InitPlanPreview{
		InvocationPolicy: plan.intent.policy,
		Readiness:        plan.readiness,
		Core: CorePlanPreview{
			ProjectRoot:              plan.core.projectRoot,
			ProjectID:                plan.core.projectID.String(),
			DatabasePath:             plan.core.databasePath,
			DatabaseSeedKind:         plan.core.databaseSeed.kind,
			DatabaseSeedObservedPath: plan.core.databaseSeed.observationPath,
			DatabaseSeedPath:         plan.core.databaseSeed.sourcePath,
			DatabaseSeedHash:         plan.core.databaseSeed.digest,
			RootMigrationKind:        plan.core.rootMigration.kind,
			RootMigrationSource:      plan.core.rootMigration.source,
			RootMigrationTarget:      plan.core.rootMigration.target,
			Effect:                   plan.core.effect,
			BeforeSchema:             plan.core.beforeSchema,
			AfterSchema:              plan.core.afterSchema,
			BasisKind:                basis.kind,
			BasisReason:              basis.reason,
			HeadRef:                  basis.headRef,
			HeadRevision:             basis.headRevision,
			SelectedComposite:        basis.compositeRef,
			FileEffects:              coreFiles,
		},
		Hosts:                 hosts,
		ApplyOrder:            []string{"core_project", "host_adapter_fanout"},
		PartialEffectBoundary: "core_applied_host_incomplete",
	}
}

func previewHostPlan(plan HostAdapterInstallPlan) HostPlanPreview {
	conflicts := make(map[string]InstallConflict, len(plan.conflicts))
	for _, conflict := range plan.conflicts {
		conflicts[conflict.path] = conflict
	}
	effects := make(
		[]FileEffectPreview,
		0,
		len(plan.outputs)+
			len(plan.removals)+
			len(plan.conflicts)+
			len(plan.managedCarriers),
	)
	seen := make(map[string]struct{}, cap(effects))
	for index, output := range plan.outputs {
		expectation := plan.expectations[index]
		effect := previewOutputEffect(expectation, output)
		if conflict, blocked := conflicts[output.path]; blocked {
			effect.Effect = FileConflict
			effect.ConflictKind = conflict.kind
			effect.Reason = conflict.reason
			effect.OwnershipKind = conflict.basis.kind
			effect.OwnershipRef = conflict.basis.ref
			effect.OwnershipDigest = conflict.basis.digest
		}
		effects = append(effects, effect)
		seen[output.path] = struct{}{}
	}
	for _, removal := range plan.removals {
		expectation := removal.expectation
		effect := FileEffectPreview{
			Path:               expectation.path,
			Effect:             FileRemoveOwnedOrphan,
			PredecessorKind:    expectation.kind,
			PredecessorDigest:  expectation.digest,
			PredecessorMode:    uint32(expectation.mode.Perm()),
			ManifestPathDigest: expectation.manifestDigest,
			ManifestPathMode:   uint32(expectation.manifestMode.Perm()),
			OwnershipKind:      expectation.basis.kind,
			OwnershipRef:       expectation.basis.ref,
			OwnershipDigest:    expectation.basis.digest,
		}
		if conflict, blocked := conflicts[expectation.path]; blocked {
			effect.Effect = FileConflict
			effect.ConflictKind = conflict.kind
			effect.Reason = conflict.reason
			effect.OwnershipKind = conflict.basis.kind
			effect.OwnershipRef = conflict.basis.ref
			effect.OwnershipDigest = conflict.basis.digest
		}
		effects = append(effects, effect)
		seen[expectation.path] = struct{}{}
	}
	for _, conflict := range plan.conflicts {
		if _, included := seen[conflict.path]; included {
			continue
		}
		effects = append(effects, FileEffectPreview{
			Path:            conflict.path,
			Effect:          FileConflict,
			ConflictKind:    conflict.kind,
			Reason:          conflict.reason,
			OwnershipKind:   conflict.basis.kind,
			OwnershipRef:    conflict.basis.ref,
			OwnershipDigest: conflict.basis.digest,
		})
	}
	managedFragments := []ManagedFragmentEffectPreview{}
	for _, carrier := range plan.managedCarriers {
		effects = append(
			effects,
			previewManagedCarrierEffect(carrier),
		)
		for _, effect := range carrier.Effects() {
			managedFragments = append(
				managedFragments,
				previewManagedFragmentEffect(effect),
			)
		}
		for _, conflict := range carrier.Conflicts() {
			managedFragments = append(
				managedFragments,
				previewManagedFragmentConflict(conflict),
			)
		}
	}
	sort.Slice(effects, func(left int, right int) bool {
		return effects[left].Path < effects[right].Path
	})
	sort.Slice(managedFragments, func(left int, right int) bool {
		return managedFragmentPreviewKey(managedFragments[left]) <
			managedFragmentPreviewKey(managedFragments[right])
	})
	return HostPlanPreview{
		Host:                plan.host,
		Edition:             plan.edition,
		Scope:               plan.scope,
		Components:          plan.components.Values(),
		TargetRoots:         slices.Clone(plan.targetRoots),
		HaftVersion:         plan.publication.haftVersion,
		ExecutablePath:      plan.publication.executablePath,
		ExecutableDigest:    plan.publication.executableDigest,
		SkillBundleDigest:   plan.publication.skillBundleDigest,
		KernelCatalogDigest: plan.publication.kernelCatalogDigest,
		Effects:             effects,
		ManagedFragments:    managedFragments,
		RecoveryArgv:        plan.recovery.Argv(),
	}
}

func previewManagedCarrierEffect(
	carrier ManagedCarrierInstallPlan,
) FileEffectPreview {
	predecessor := carrier.Predecessor()
	preview := FileEffectPreview{
		Path:              carrier.Path(),
		Component:         carrier.Component(),
		Components:        carrier.Components().Values(),
		SharedCarrier:     true,
		PredecessorDigest: predecessor.Digest(),
		PredecessorMode:   uint32(predecessor.Mode().Perm()),
	}
	if predecessor.Kind() == ManagedCarrierMissing {
		preview.PredecessorKind = PredecessorMissing
	}
	if predecessor.Kind() == ManagedCarrierPresent {
		preview.PredecessorKind = PredecessorSharedCarrierExact
	}
	conflicts := carrier.Conflicts()
	if len(conflicts) != 0 {
		preview.Effect = FileConflict
		preview.Reason = conflicts[0].Reason()
		basis := conflicts[0].OwnershipBasis()
		preview.OwnershipKind = basis.kind
		preview.OwnershipRef = basis.ref
		preview.OwnershipDigest = basis.digest
		return preview
	}
	result, available := carrier.MutationResult()
	if !available {
		preview.Effect = FileConflict
		preview.Reason = "managed carrier has no terminal projection"
		return preview
	}
	preview.RenderedDigest = result.Digest()
	preview.RenderedMode = uint32(result.Mode().Perm())
	preview.Effect = FilePreserve
	if result.Changed() && predecessor.Kind() == ManagedCarrierMissing {
		preview.Effect = FileCreate
	}
	if result.Changed() && predecessor.Kind() == ManagedCarrierPresent {
		preview.Effect = FileUpdate
	}
	return preview
}

func previewManagedFragmentEffect(
	effect ManagedFragmentEffect,
) ManagedFragmentEffectPreview {
	coordinate := effect.Coordinate()
	desired, hasDesired := effect.Desired()
	desiredDigest := ""
	if hasDesired {
		desiredDigest = desired.Digest()
	}
	return ManagedFragmentEffectPreview{
		CarrierPath:    coordinate.CarrierPath(),
		Component:      effect.Component(),
		Kind:           coordinate.Kind(),
		Selector:       coordinate.Selector(),
		MemberID:       coordinate.MemberID(),
		MergeEdition:   coordinate.MergeEdition(),
		Effect:         effect.Kind(),
		ExpectedKind:   effect.ExpectedKind(),
		ExpectedDigest: effect.ExpectedDigest(),
		DesiredDigest:  desiredDigest,
	}
}

func previewManagedFragmentConflict(
	conflict ManagedFragmentConflict,
) ManagedFragmentEffectPreview {
	coordinate := conflict.Coordinate()
	basis := conflict.OwnershipBasis()
	return ManagedFragmentEffectPreview{
		CarrierPath:     coordinate.CarrierPath(),
		Component:       conflict.Component(),
		Kind:            coordinate.Kind(),
		Selector:        coordinate.Selector(),
		MemberID:        coordinate.MemberID(),
		MergeEdition:    coordinate.MergeEdition(),
		ConflictKind:    conflict.Kind(),
		Reason:          conflict.Reason(),
		OwnershipKind:   basis.kind,
		OwnershipRef:    basis.ref,
		OwnershipDigest: basis.digest,
	}
}

func managedFragmentPreviewKey(
	preview ManagedFragmentEffectPreview,
) string {
	return preview.CarrierPath + "\x00" +
		string(preview.Kind) + "\x00" +
		preview.Selector + "\x00" +
		preview.MemberID + "\x00" +
		preview.MergeEdition
}

func previewOutputEffect(
	expectation PathExpectation,
	output RenderedOutput,
) FileEffectPreview {
	effect := FileUpdate
	switch expectation.kind {
	case PredecessorMissing, PredecessorMissingOwned:
		effect = FileCreate
	case PredecessorCurrentOwned:
		effect = FilePreserve
	case PredecessorOutdatedOwned:
		effect = FileUpdate
	case PredecessorKnownLegacyExact:
		effect = FileUpdateLegacy
		if expectation.digest == output.digest {
			effect = FileAdoptLegacy
		}
	}
	return FileEffectPreview{
		Path:               output.path,
		Component:          output.Component(),
		Effect:             effect,
		PredecessorKind:    expectation.kind,
		PredecessorDigest:  expectation.digest,
		PredecessorMode:    uint32(expectation.mode.Perm()),
		ManifestPathDigest: expectation.manifestDigest,
		ManifestPathMode:   uint32(expectation.manifestMode.Perm()),
		OwnershipKind:      expectation.basis.kind,
		OwnershipRef:       expectation.basis.ref,
		OwnershipDigest:    expectation.basis.digest,
		RenderedDigest:     output.digest,
		RenderedMode:       uint32(output.mode.Perm()),
	}
}
