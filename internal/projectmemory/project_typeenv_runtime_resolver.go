package projectmemory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstagerevalidation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

var (
	ErrProjectTypeEnvRuntimeResolverDependencies = errors.New(
		"project-memory TypeEnv runtime resolver dependencies are incomplete",
	)
	ErrProjectTypeEnvRuntimeUnavailable = errors.New(
		"project-memory selected TypeEnv runtime is unavailable",
	)
	ErrProjectTypeEnvRuntimeUncorrelated = errors.New(
		"project-memory selected TypeEnv runtime is uncorrelated",
	)
)

// CurrentProjectTypeEnvSelectionClosureLoader is the read-only effect-owned
// port for recovering one exact committed closure. Implementations must verify
// the complete stored closure DAG; this consumer does not duplicate replay SQL.
type CurrentProjectTypeEnvSelectionClosureLoader interface {
	LoadCommittedClosureForCurrentHeadTx(
		context.Context,
		*sqlitetransaction.Transaction,
		projectidentity.ProjectID,
		typedmemory.GraphRevision,
		projecttypeenvselection.ProjectTypeEnvHeadRef,
		projecttypeenvselection.HeadRevision,
		typedmemory.TypeEnvRef,
	) (
		projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
		error,
	)
}

// ProjectTypeEnvRuntimeResolver reconstructs the current selected project
// TypeEnv inside the transaction owned by typedmemorystore. It has no head,
// Stage, activation, or transaction mutation port.
type ProjectTypeEnvRuntimeResolver struct {
	stages            *projecttypeenvstage.Store
	heads             *projecttypeenvheadstore.Store
	closures          CurrentProjectTypeEnvSelectionClosureLoader
	installedRuntimes InstalledProjectTypeEnvRuntimeCatalog
}

var _ typedmemorystore.SelectedProjectTypeEnvRuntimeResolver = (*ProjectTypeEnvRuntimeResolver)(nil)

func NewProjectTypeEnvRuntimeResolver(
	stages *projecttypeenvstage.Store,
	heads *projecttypeenvheadstore.Store,
	closures CurrentProjectTypeEnvSelectionClosureLoader,
	installedRuntimes InstalledProjectTypeEnvRuntimeCatalog,
) (*ProjectTypeEnvRuntimeResolver, error) {
	if stages == nil ||
		heads == nil ||
		!interfaceValuePresent(closures) ||
		installedRuntimes.Len() == 0 {
		return nil, ErrProjectTypeEnvRuntimeResolverDependencies
	}
	return &ProjectTypeEnvRuntimeResolver{
		stages:            stages,
		heads:             heads,
		closures:          closures,
		installedRuntimes: installedRuntimes,
	}, nil
}

func (resolver *ProjectTypeEnvRuntimeResolver) ResolveSelectedProjectTypeEnvRuntime(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request typedmemorystore.SelectedProjectTypeEnvRuntimeRequest,
) (typedmemorystore.SelectedProjectTypeEnvRuntime, error) {
	if ctx == nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{},
			fmt.Errorf("resolve project TypeEnv runtime: context is required")
	}
	if resolver == nil ||
		resolver.stages == nil ||
		resolver.heads == nil ||
		!interfaceValuePresent(resolver.closures) ||
		resolver.installedRuntimes.Len() == 0 {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{},
			ErrProjectTypeEnvRuntimeResolverDependencies
	}
	if transaction == nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{},
			sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireActive(); err != nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{}, err
	}

	head, err := resolver.loadCorrelatedCurrentHead(ctx, transaction, request)
	if err != nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{}, err
	}
	closure, err := resolver.closures.LoadCommittedClosureForCurrentHeadTx(
		ctx,
		transaction,
		request.Project(),
		request.GraphRevision(),
		head.Ref(),
		head.Revision(),
		request.SelectedTypeEnv(),
	)
	if err != nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{},
			fmt.Errorf("load committed project TypeEnv selection closure: %w", err)
	}
	if err := verifyCurrentClosureCoordinates(request, head, closure); err != nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{}, err
	}
	ready, err := resolver.stages.LoadSelectionReadyTx(
		ctx,
		transaction,
		closure.Target().Stage(),
	)
	if err != nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{},
			fmt.Errorf("reload selected project TypeEnv Stage: %w", err)
	}
	if err := verifySelectionReadyRuntimeCoordinates(
		request,
		closure,
		ready,
	); err != nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{}, err
	}
	runtimeBasis, err := projecttypeenvstore.GetRuntimeEvaluationBasisArtifactTx(
		ctx,
		transaction,
		ready.Stage().RuntimeBasis(),
	)
	if err != nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{},
			fmt.Errorf("reload selected project TypeEnv runtime basis X: %w", err)
	}
	installed, present := resolver.installedRuntimes.Lookup(runtimeBasis.Ref())
	if !present {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{},
			fmt.Errorf(
				"%w: selected X %q has no exact installed runtime",
				ErrProjectTypeEnvRuntimeUnavailable,
				runtimeBasis.Ref().String(),
			)
	}
	registry, err := matchInstalledProjectTypeEnvRuntime(
		runtimeBasis,
		installed,
	)
	if err != nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{}, err
	}
	codecs, found := registry.CodecRegistry()
	if !found {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{},
			fmt.Errorf(
				"%w: exact target runtime exposed no CodecRegistry",
				ErrProjectTypeEnvRuntimeUnavailable,
			)
	}
	memberOf, err := selectedMemberOfRuntime(
		runtimeBasis,
		registry,
	)
	if err != nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{}, err
	}
	classification, err := selectedKindClassificationRuntime(
		runtimeBasis,
		registry,
	)
	if err != nil {
		return typedmemorystore.SelectedProjectTypeEnvRuntime{}, err
	}
	builder := typedmemorystore.NewSelectedProjectTypeEnvRuntimeBuilder(request)
	return builder.
		SetEnvironment(ready.ExecutableTypeEnv()).
		SetCodecs(codecs).
		SetMemberOfRuntime(memberOf).
		SetKindClassificationRuntime(classification).
		Build()
}

func selectedKindClassificationRuntime(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	registry projecttypeenvruntime.ExactTargetRuntimeRegistry,
) (typedmemorystore.SelectedKindClassificationRuntime, error) {
	required := false
	for _, pin := range runtimeBasis.Pins() {
		if pin.InvocationContract() == projecttypeenv.RuntimeMechanismContractKindClassification {
			required = true
		}
	}
	if !required {
		return typedmemorystore.NewKindClassificationNotRequired(), nil
	}
	matchedRuntimeBasis, found := registry.RuntimeBasisRef()
	if !found || matchedRuntimeBasis != runtimeBasis.Ref() {
		return nil, fmt.Errorf(
			"%w: kind-classification evaluator registry differs from selected X",
			ErrProjectTypeEnvRuntimeUncorrelated,
		)
	}
	registryCoordinate, found := registry.CoordinateDigest()
	if !found {
		return nil, fmt.Errorf(
			"%w: exact target registry has no coordinate digest",
			ErrProjectTypeEnvRuntimeUnavailable,
		)
	}
	evaluators, found := registry.KindClassificationRegistry()
	if !found || evaluators.Len() == 0 {
		return nil, fmt.Errorf(
			"%w: exact target runtime exposed no kind-classification evaluator registry",
			ErrProjectTypeEnvRuntimeUnavailable,
		)
	}
	engine, err := NewProjectKindClassificationAdmissionEngine(evaluators)
	if err != nil {
		return nil, fmt.Errorf(
			"construct selected-C kind-classification dispatcher: %w",
			err,
		)
	}
	runtimeBasisDigest, err := typedmemorystore.NewSelectedRuntimeBasisDigest(
		runtimeBasis.Digest(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"bind selected-C runtime-basis coordinate: %w",
			err,
		)
	}
	targetRegistryDigest, err := typedmemorystore.NewExactTargetRegistryCoordinateDigest(
		registryCoordinate,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"bind selected-C target-registry coordinate: %w",
			err,
		)
	}
	exact, err := typedmemorystore.NewExactKindClassificationRuntime(
		typedmemorystore.ExactKindClassificationRuntimeInput{
			Engine:                   engine,
			RuntimeBasisDigest:       runtimeBasisDigest,
			RegistryCoordinateDigest: targetRegistryDigest,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"bind selected-C kind-classification evaluator registry: %w",
			err,
		)
	}
	return exact, nil
}

func selectedMemberOfRuntime(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	registry projecttypeenvruntime.ExactTargetRuntimeRegistry,
) (typedmemorystore.SelectedMemberOfRuntime, error) {
	required := false
	for _, pin := range runtimeBasis.Pins() {
		contract := pin.InvocationContract()
		if contract == projecttypeenv.RuntimeMechanismContractMemberOf ||
			contract == projecttypeenv.RuntimeMechanismContractCarrierMembershipDelivery {
			required = true
		}
	}
	if len(runtimeBasis.RegistrationPolicyPins()) > 0 {
		required = true
	}
	if !required {
		return typedmemorystore.NewMemberOfNotRequired(), nil
	}
	matchedRuntimeBasis, found := registry.RuntimeBasisRef()
	if !found || matchedRuntimeBasis != runtimeBasis.Ref() {
		return nil, fmt.Errorf(
			"%w: MemberOf evaluator registry differs from selected X",
			ErrProjectTypeEnvRuntimeUncorrelated,
		)
	}
	registryCoordinate, found := registry.CoordinateDigest()
	if !found {
		return nil, fmt.Errorf(
			"%w: exact target registry has no coordinate digest",
			ErrProjectTypeEnvRuntimeUnavailable,
		)
	}
	engine, err := selectedMemberOfEvaluatorRegistry(registry)
	if err != nil {
		return nil, fmt.Errorf(
			"construct selected-C MemberOf dispatcher: %w",
			err,
		)
	}
	runtimeBasisDigest, err := typedmemorystore.NewSelectedRuntimeBasisDigest(
		runtimeBasis.Digest(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"bind selected-C runtime-basis coordinate: %w",
			err,
		)
	}
	targetRegistryDigest, err :=
		typedmemorystore.NewExactTargetRegistryCoordinateDigest(
			registryCoordinate,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"bind selected-C target-registry coordinate: %w",
			err,
		)
	}
	exact, err := typedmemorystore.NewExactMemberOfRuntime(
		typedmemorystore.ExactMemberOfRuntimeInput{
			Engine:                   engine,
			RuntimeBasisDigest:       runtimeBasisDigest,
			RegistryCoordinateDigest: targetRegistryDigest,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"bind selected-C MemberOf evaluator registry: %w",
			err,
		)
	}
	return exact, nil
}

func (resolver *ProjectTypeEnvRuntimeResolver) loadCorrelatedCurrentHead(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request typedmemorystore.SelectedProjectTypeEnvRuntimeRequest,
) (projecttypeenvselection.ProjectTypeEnvHeadState, error) {
	observation, err := resolver.heads.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		transaction,
		request.Project(),
	)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{},
			fmt.Errorf("reload current project TypeEnv head: %w", err)
	}
	current, ok :=
		observation.(projecttypeenvstagerevalidation.ObservedProjectTypeEnvHead)
	if !ok {
		return projecttypeenvselection.ProjectTypeEnvHeadState{},
			fmt.Errorf(
				"%w: project-executable C has no current dedicated head",
				ErrProjectTypeEnvRuntimeUnavailable,
			)
	}
	state := current.State()
	if state.Project() != request.Project() ||
		state.SelectedComposite() != request.SelectedTypeEnv() {
		return projecttypeenvselection.ProjectTypeEnvHeadState{},
			fmt.Errorf(
				"%w: current dedicated head differs from requested project/C",
				ErrProjectTypeEnvRuntimeUncorrelated,
			)
	}
	return state, nil
}

func verifyCurrentClosureCoordinates(
	request typedmemorystore.SelectedProjectTypeEnvRuntimeRequest,
	head projecttypeenvselection.ProjectTypeEnvHeadState,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) error {
	successor := closure.SuccessorHead()
	matches := closure.Project() == request.Project() &&
		closure.CommittedGraphRevision().Value() <= request.GraphRevision().Value() &&
		closure.Target().Composite() == request.SelectedTypeEnv() &&
		successor.Project() == head.Project() &&
		successor.Ref() == head.Ref() &&
		successor.Revision() == head.Revision() &&
		successor.SelectedComposite() == head.SelectedComposite() &&
		bytes.Equal(successor.CanonicalBytes(), head.CanonicalBytes())
	if !matches {
		return fmt.Errorf(
			"%w: current closure differs from project/C/head or was committed after the requested graph revision",
			ErrProjectTypeEnvRuntimeUncorrelated,
		)
	}
	return nil
}

func verifySelectionReadyRuntimeCoordinates(
	request typedmemorystore.SelectedProjectTypeEnvRuntimeRequest,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
	ready projecttypeenvstage.SelectionReadyStage,
) error {
	if err := ready.Verify(); err != nil {
		return fmt.Errorf("verify selected project TypeEnv Stage: %w", err)
	}
	stage := ready.Stage()
	target := closure.Target()
	matches := stage.Project() == request.Project() &&
		stage.Ref() == target.Stage() &&
		stage.Base() == target.Base() &&
		slices.Equal(stage.OrderedExtensions(), target.OrderedExtensions()) &&
		stage.RuntimeBasis() == target.RuntimeBasis() &&
		stage.VerifiedComposite() == target.Composite() &&
		ready.ExecutableTypeEnv().Ref() == request.SelectedTypeEnv()
	if !matches {
		return fmt.Errorf(
			"%w: reloaded Stage B/E/X/C differs from selected closure",
			ErrProjectTypeEnvRuntimeUncorrelated,
		)
	}
	return nil
}

func matchInstalledProjectTypeEnvRuntime(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	installed projecttypeenvruntime.InstalledRuntimeRegistryInput,
) (projecttypeenvruntime.ExactTargetRuntimeRegistry, error) {
	resolution := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: runtimeBasis,
			Installed:    installed,
		},
	)
	matched, ok := resolution.(projecttypeenvruntime.Matched)
	if !ok {
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{},
			fmt.Errorf(
				"%w: target X does not match installed runtime (%s)",
				ErrProjectTypeEnvRuntimeUnavailable,
				resolution.Kind().String(),
			)
	}
	registry, found := matched.Registry()
	if !found || !registry.Valid() {
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{},
			fmt.Errorf(
				"%w: matched target X exposed no exact runtime registry",
				ErrProjectTypeEnvRuntimeUnavailable,
			)
	}
	runtimeRef, found := registry.RuntimeBasisRef()
	if !found || runtimeRef != runtimeBasis.Ref() {
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{},
			fmt.Errorf(
				"%w: matched runtime registry differs from selected X",
				ErrProjectTypeEnvRuntimeUncorrelated,
			)
	}
	return registry, nil
}

func cloneInstalledRuntimeRegistry(
	input projecttypeenvruntime.InstalledRuntimeRegistryInput,
) projecttypeenvruntime.InstalledRuntimeRegistryInput {
	return projecttypeenvruntime.InstalledRuntimeRegistryInput{
		Codecs:                                   input.Codecs,
		EntitySetEnumerationEvaluators:           input.EntitySetEnumerationEvaluators.Clone(),
		CandidateVisibilityEvaluators:            input.CandidateVisibilityEvaluators.Clone(),
		KindDefinednessEvaluators:                input.KindDefinednessEvaluators.Clone(),
		MemberOfEvaluators:                       input.MemberOfEvaluators.Clone(),
		KindClassificationEvaluators:             input.KindClassificationEvaluators.Clone(),
		ReferenceDesignationResolutionEvaluators: input.ReferenceDesignationResolutionEvaluators.Clone(),
		ClaimInterpretationEvaluators:            input.ClaimInterpretationEvaluators.Clone(),
		ClaimMeasurementEvaluators:               input.ClaimMeasurementEvaluators.Clone(),
		ClaimEvaluationEvaluators:                input.ClaimEvaluationEvaluators.Clone(),
		EpistemeConstitutionEvaluators:           input.EpistemeConstitutionEvaluators.Clone(),
		MechanismCatalogs: append(
			[]runtimemechanism.RuntimeMechanismArtifactV1(nil),
			input.MechanismCatalogs...,
		),
		RegistrationPolicies: append(
			[]recordmembershipregistration.RegistrationArtifactV1(nil),
			input.RegistrationPolicies...,
		),
	}
}
