package initplanning

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
)

type ManifestPredecessorKind string

const (
	ManifestPredecessorMissing ManifestPredecessorKind = "missing"
	ManifestPredecessorExact   ManifestPredecessorKind = "exact"
)

type ManifestPredecessor struct {
	kind   ManifestPredecessorKind
	ref    string
	digest string
}

func (predecessor ManifestPredecessor) Kind() ManifestPredecessorKind {
	return predecessor.kind
}

func (predecessor ManifestPredecessor) Ref() string {
	return predecessor.ref
}

func (predecessor ManifestPredecessor) Digest() string {
	return predecessor.digest
}

type HostPublicationStepKind string

const (
	PublicationPreserve    HostPublicationStepKind = "preserve"
	PublicationAdoptLegacy HostPublicationStepKind = "adopt_known_legacy"
	PublicationCreate      HostPublicationStepKind = "create"
	PublicationReplace     HostPublicationStepKind = "replace"
	PublicationRemove      HostPublicationStepKind = "remove"
)

type HostPublicationStep struct {
	kind        HostPublicationStepKind
	components  ComponentSet
	expectation PathExpectation
	output      RenderedOutput
	hasOutput   bool
	managed     bool
}

func (step HostPublicationStep) Kind() HostPublicationStepKind {
	return step.kind
}

func (step HostPublicationStep) Path() string {
	return step.expectation.path
}

func (step HostPublicationStep) Component() Component {
	component, _ := step.components.single()
	return component
}

func (step HostPublicationStep) Components() ComponentSet {
	return ComponentSet{values: step.components.Values()}
}

func (step HostPublicationStep) Expectation() PathExpectation {
	return step.expectation
}

func (step HostPublicationStep) Output() (RenderedOutput, bool) {
	if !step.hasOutput {
		return RenderedOutput{}, false
	}
	return cloneRenderedOutputs([]RenderedOutput{step.output})[0], true
}

func (step HostPublicationStep) IsManagedCarrier() bool {
	return step.managed
}

type HostPublicationBatch struct {
	host                HostID
	edition             string
	projectRoot         string
	projectID           string
	scope               InstallScope
	targetRoots         []string
	steps               []HostPublicationStep
	manifest            InstallationManifest
	manifestPredecessor ManifestPredecessor
	recovery            RecoveryOperation
	canonical           []byte
	digest              string
}

func BuildHostPublicationBatch(
	plan HostAdapterInstallPlan,
) (HostPublicationBatch, error) {
	if len(plan.conflicts) != 0 ||
		len(plan.ManagedFragmentConflicts()) != 0 {
		return HostPublicationBatch{}, fmt.Errorf("blocked host adapter plan cannot become a publication batch")
	}
	manifest, err := BuildInstallationManifest(plan)
	if err != nil {
		return HostPublicationBatch{}, err
	}
	steps := make(
		[]HostPublicationStep,
		0,
		len(plan.outputs)+len(plan.removals)+len(plan.managedCarriers),
	)
	for index, output := range plan.outputs {
		step, err := publicationOutputStep(plan.expectations[index], output)
		if err != nil {
			return HostPublicationBatch{}, err
		}
		steps = append(steps, step)
	}
	for _, removal := range plan.removals {
		components, err := singletonComponentSet(removal.component)
		if err != nil {
			return HostPublicationBatch{}, err
		}
		steps = append(steps, HostPublicationStep{
			kind:        PublicationRemove,
			components:  components,
			expectation: removal.expectation,
		})
	}
	for _, carrier := range plan.managedCarriers {
		step, err := publicationManagedCarrierStep(carrier)
		if err != nil {
			return HostPublicationBatch{}, err
		}
		steps = append(steps, step)
	}
	sort.Slice(steps, func(left int, right int) bool {
		return steps[left].Path() < steps[right].Path()
	})
	predecessor, err := publicationManifestPredecessor(
		steps,
		plan.manifestBasis,
	)
	if err != nil {
		return HostPublicationBatch{}, err
	}
	batch := HostPublicationBatch{
		host:                plan.host,
		edition:             plan.edition,
		projectRoot:         plan.projectRoot,
		projectID:           plan.projectID.String(),
		scope:               plan.scope,
		targetRoots:         slices.Clone(plan.targetRoots),
		steps:               cloneHostPublicationSteps(steps),
		manifest:            cloneInstallationManifest(manifest),
		manifestPredecessor: predecessor,
		recovery:            RecoveryOperation{argv: plan.recovery.Argv()},
	}
	canonical, err := canonicalHostPublicationBatch(batch)
	if err != nil {
		return HostPublicationBatch{}, err
	}
	batch.canonical = canonical
	batch.digest = digestBytesForManifest(canonical)
	return batch, nil
}

func publicationOutputStep(
	expectation PathExpectation,
	output RenderedOutput,
) (HostPublicationStep, error) {
	if !expectation.valid() || expectation.path != output.path {
		return HostPublicationStep{}, fmt.Errorf("publication output lacks its exact predecessor")
	}
	kind := PublicationReplace
	switch expectation.kind {
	case PredecessorMissing, PredecessorMissingOwned:
		kind = PublicationCreate
	case PredecessorCurrentOwned:
		if expectation.digest != output.digest || expectation.mode != output.mode {
			return HostPublicationStep{}, fmt.Errorf("current-owned publication output differs from observed predecessor")
		}
		kind = PublicationPreserve
	case PredecessorOutdatedOwned:
		if expectation.digest == output.digest && expectation.mode == output.mode {
			return HostPublicationStep{}, fmt.Errorf("outdated-owned publication output is already current")
		}
	case PredecessorKnownLegacyExact:
		if expectation.digest == output.digest && expectation.mode == output.mode {
			kind = PublicationAdoptLegacy
		}
	case PredecessorLocallyModifiedOwned, PredecessorForeign, PredecessorOrphanedOwned:
		return HostPublicationStep{}, fmt.Errorf(
			"predecessor %s cannot enter an unblocked publication output",
			expectation.kind,
		)
	default:
		return HostPublicationStep{}, fmt.Errorf("publication predecessor kind is invalid")
	}
	return HostPublicationStep{
		kind:        kind,
		components:  output.Components(),
		expectation: expectation,
		output:      cloneRenderedOutputs([]RenderedOutput{output})[0],
		hasOutput:   true,
	}, nil
}

func publicationManagedCarrierStep(
	carrier ManagedCarrierInstallPlan,
) (HostPublicationStep, error) {
	if carrier.Readiness() != ManagedCarrierReady {
		return HostPublicationStep{}, fmt.Errorf(
			"blocked managed carrier cannot become a publication step",
		)
	}
	predecessor := carrier.Predecessor()
	result, available := carrier.MutationResult()
	if !available || result.Kind() == ManagedCarrierAbsent {
		return HostPublicationStep{}, fmt.Errorf(
			"managed carrier publication has no materialized terminal carrier",
		)
	}
	output, err := NewRenderedOutputForComponents(
		result.Path(),
		carrier.Components(),
		result.Content(),
		result.Mode(),
	)
	if err != nil {
		return HostPublicationStep{}, err
	}
	var expectation PathExpectation
	switch predecessor.Kind() {
	case ManagedCarrierMissing:
		expectation, err = ExpectMissing(predecessor.Path())
	case ManagedCarrierPresent:
		expectation, err = ExpectSharedCarrierExact(
			predecessor.Path(),
			predecessor.Digest(),
			predecessor.Mode(),
		)
	default:
		err = fmt.Errorf("managed carrier predecessor kind is invalid")
	}
	if err != nil {
		return HostPublicationStep{}, err
	}
	kind := PublicationReplace
	if predecessor.Kind() == ManagedCarrierMissing {
		kind = PublicationCreate
	}
	if !result.Changed() {
		kind = PublicationPreserve
	}
	return HostPublicationStep{
		kind:        kind,
		components:  carrier.Components(),
		expectation: expectation,
		output:      output,
		hasOutput:   true,
		managed:     true,
	}, nil
}

func publicationManifestPredecessor(
	steps []HostPublicationStep,
	explicit OwnershipBasis,
) (ManifestPredecessor, error) {
	var predecessor ManifestPredecessor
	if explicit.valid() {
		if explicit.kind != OwnershipManifestReceipt {
			return ManifestPredecessor{}, fmt.Errorf(
				"publication manifest predecessor basis is invalid",
			)
		}
		predecessor = ManifestPredecessor{
			kind:   ManifestPredecessorExact,
			ref:    explicit.ref,
			digest: explicit.digest,
		}
	}
	for _, step := range steps {
		basis := step.expectation.basis
		if basis.kind != OwnershipManifestReceipt {
			continue
		}
		if predecessor.kind == "" {
			predecessor = ManifestPredecessor{
				kind:   ManifestPredecessorExact,
				ref:    basis.ref,
				digest: basis.digest,
			}
			continue
		}
		if predecessor.ref != basis.ref || predecessor.digest != basis.digest {
			return ManifestPredecessor{}, fmt.Errorf("publication batch mixes manifest predecessors")
		}
	}
	if predecessor.kind == "" {
		return ManifestPredecessor{kind: ManifestPredecessorMissing}, nil
	}
	return predecessor, nil
}

func cloneHostPublicationSteps(source []HostPublicationStep) []HostPublicationStep {
	result := make([]HostPublicationStep, len(source))
	for index, step := range source {
		result[index] = step
		result[index].components = step.Components()
		if step.hasOutput {
			result[index].output = cloneRenderedOutputs([]RenderedOutput{step.output})[0]
		}
	}
	return result
}

func (batch HostPublicationBatch) Host() HostID {
	return batch.host
}

func (batch HostPublicationBatch) Edition() string {
	return batch.edition
}

func (batch HostPublicationBatch) ProjectRoot() string {
	return batch.projectRoot
}

func (batch HostPublicationBatch) ProjectID() string {
	return batch.projectID
}

func (batch HostPublicationBatch) Scope() InstallScope {
	return batch.scope
}

func (batch HostPublicationBatch) TargetRoots() []string {
	return slices.Clone(batch.targetRoots)
}

func (batch HostPublicationBatch) Steps() []HostPublicationStep {
	return cloneHostPublicationSteps(batch.steps)
}

func (batch HostPublicationBatch) Manifest() InstallationManifest {
	return cloneInstallationManifest(batch.manifest)
}

func (batch HostPublicationBatch) ManifestPredecessor() ManifestPredecessor {
	return batch.manifestPredecessor
}

func (batch HostPublicationBatch) Recovery() RecoveryOperation {
	return RecoveryOperation{argv: batch.recovery.Argv()}
}

func (batch HostPublicationBatch) Digest() string {
	return batch.digest
}

type publicationBatchStepWire struct {
	Kind                  HostPublicationStepKind `json:"kind"`
	Path                  string                  `json:"path"`
	Component             Component               `json:"component,omitempty"`
	Components            []Component             `json:"components,omitempty"`
	PredecessorKind       PredecessorKind         `json:"predecessor_kind"`
	PredecessorDigest     string                  `json:"predecessor_digest,omitempty"`
	PredecessorMode       uint32                  `json:"predecessor_mode,omitempty"`
	ManifestPathDigest    string                  `json:"manifest_path_digest,omitempty"`
	ManifestPathMode      uint32                  `json:"manifest_path_mode,omitempty"`
	OwnershipKind         OwnershipBasisKind      `json:"ownership_kind,omitempty"`
	OwnershipRef          string                  `json:"ownership_ref,omitempty"`
	OwnershipDigest       string                  `json:"ownership_digest,omitempty"`
	RenderedDigest        string                  `json:"rendered_digest,omitempty"`
	RenderedMode          uint32                  `json:"rendered_mode,omitempty"`
	RenderedContentDigest string                  `json:"rendered_content_digest,omitempty"`
	ManagedCarrier        bool                    `json:"managed_carrier,omitempty"`
}

type publicationBatchWire struct {
	Schema                    string                     `json:"schema"`
	Host                      HostID                     `json:"host"`
	AdapterEdition            string                     `json:"adapter_edition"`
	ProjectRoot               string                     `json:"project_root"`
	ProjectID                 string                     `json:"project_id"`
	Scope                     InstallScope               `json:"scope"`
	TargetRoots               []string                   `json:"target_roots"`
	DesiredManifestDigest     string                     `json:"desired_manifest_digest"`
	ManifestPredecessorKind   ManifestPredecessorKind    `json:"manifest_predecessor_kind"`
	ManifestPredecessorRef    string                     `json:"manifest_predecessor_ref,omitempty"`
	ManifestPredecessorDigest string                     `json:"manifest_predecessor_digest,omitempty"`
	RecoveryArgv              []string                   `json:"recovery_argv"`
	Steps                     []publicationBatchStepWire `json:"steps"`
}

func canonicalHostPublicationBatch(
	batch HostPublicationBatch,
) ([]byte, error) {
	steps := make([]publicationBatchStepWire, len(batch.steps))
	for index, step := range batch.steps {
		basis := step.expectation.basis
		component, singleton := step.components.single()
		wire := publicationBatchStepWire{
			Kind:               step.kind,
			Path:               step.Path(),
			PredecessorKind:    step.expectation.kind,
			PredecessorDigest:  step.expectation.digest,
			PredecessorMode:    uint32(step.expectation.mode.Perm()),
			ManifestPathDigest: step.expectation.manifestDigest,
			ManifestPathMode:   uint32(step.expectation.manifestMode.Perm()),
			OwnershipKind:      basis.kind,
			OwnershipRef:       basis.ref,
			OwnershipDigest:    basis.digest,
			ManagedCarrier:     step.managed,
		}
		if singleton {
			wire.Component = component
		}
		if !singleton {
			wire.Components = step.components.Values()
		}
		if step.hasOutput {
			wire.RenderedDigest = step.output.digest
			wire.RenderedMode = uint32(step.output.mode.Perm())
			wire.RenderedContentDigest = digestBytesForManifest(step.output.content)
		}
		steps[index] = wire
	}
	schema := "haft.host-publication-batch/v1"
	for _, step := range batch.steps {
		if step.managed {
			schema = "haft.host-publication-batch/v2"
		}
		if _, singleton := step.components.single(); !singleton {
			schema = "haft.host-publication-batch/v3"
			break
		}
	}
	wire := publicationBatchWire{
		Schema:                    schema,
		Host:                      batch.host,
		AdapterEdition:            batch.edition,
		ProjectRoot:               batch.projectRoot,
		ProjectID:                 batch.projectID,
		Scope:                     batch.scope,
		TargetRoots:               slices.Clone(batch.targetRoots),
		DesiredManifestDigest:     batch.manifest.Digest(),
		ManifestPredecessorKind:   batch.manifestPredecessor.kind,
		ManifestPredecessorRef:    batch.manifestPredecessor.ref,
		ManifestPredecessorDigest: batch.manifestPredecessor.digest,
		RecoveryArgv:              batch.recovery.Argv(),
		Steps:                     steps,
	}
	canonical, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode host publication batch identity: %w", err)
	}
	return canonical, nil
}

func BuildHostPublicationObservationPlan(
	batch HostPublicationBatch,
) (InstallationObservationPlan, error) {
	if len(batch.targetRoots) == 0 || len(batch.steps) == 0 {
		return InstallationObservationPlan{}, fmt.Errorf("publication batch is invalid")
	}
	targets := make([]ObservationTarget, len(batch.steps))
	for index, step := range batch.steps {
		target := ObservationTarget{
			path:        step.Path(),
			components:  step.Components(),
			requirement: ObservationRequired,
		}
		if err := validateObservationTarget(target, batch.targetRoots); err != nil {
			return InstallationObservationPlan{}, err
		}
		targets[index] = target
	}
	return InstallationObservationPlan{
		managedRoots: slices.Clone(batch.targetRoots),
		targets:      targets,
	}, nil
}

type HostPublicationAdmissionKind string

const (
	HostPublicationPreconditionsMatched HostPublicationAdmissionKind = "matched"
	HostPublicationPreconditionsChanged HostPublicationAdmissionKind = "changed"
)

type PathPreconditionChange struct {
	expected PathExpectation
	observed PathObservation
}

func (change PathPreconditionChange) Path() string {
	return change.expected.path
}

func (change PathPreconditionChange) Expected() PathExpectation {
	return change.expected
}

func (change PathPreconditionChange) Observed() PathObservation {
	return change.observed
}

type HostPublicationAdmission struct {
	kind    HostPublicationAdmissionKind
	changes []PathPreconditionChange
}

func (admission HostPublicationAdmission) Kind() HostPublicationAdmissionKind {
	return admission.kind
}

func (admission HostPublicationAdmission) Changes() []PathPreconditionChange {
	return slices.Clone(admission.changes)
}

func ValidateHostPublicationPreconditions(
	batch HostPublicationBatch,
	observations []PathObservation,
) (HostPublicationAdmission, error) {
	if len(observations) != len(batch.steps) {
		return HostPublicationAdmission{}, fmt.Errorf("publication precondition evidence is incomplete")
	}
	byPath := make(map[string]PathObservation, len(observations))
	for _, observation := range observations {
		if _, duplicate := byPath[observation.path]; duplicate {
			return HostPublicationAdmission{}, fmt.Errorf(
				"publication precondition evidence repeats %s",
				observation.path,
			)
		}
		byPath[observation.path] = observation
	}
	changes := make([]PathPreconditionChange, 0)
	for _, step := range batch.steps {
		observation, exists := byPath[step.Path()]
		if !exists ||
			!observation.components.equal(step.components) {
			return HostPublicationAdmission{}, fmt.Errorf(
				"publication precondition evidence lacks exact component set for %s",
				step.Path(),
			)
		}
		if expectationMatchesObservation(step.expectation, observation) {
			continue
		}
		changes = append(changes, PathPreconditionChange{
			expected: step.expectation,
			observed: observation,
		})
	}
	if len(changes) != 0 {
		return HostPublicationAdmission{
			kind:    HostPublicationPreconditionsChanged,
			changes: changes,
		}, nil
	}
	return HostPublicationAdmission{
		kind: HostPublicationPreconditionsMatched,
	}, nil
}

func expectationMatchesObservation(
	expectation PathExpectation,
	observation PathObservation,
) bool {
	return expectation.MatchesObservation(observation)
}
