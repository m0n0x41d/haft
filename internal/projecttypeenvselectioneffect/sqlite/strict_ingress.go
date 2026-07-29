package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// StrictGenesisIngressResolution carries one already-durable human source into
// SelectGenesis. Captured and Replayed describe only source persistence; they
// do not claim that the ProjectTypeEnv head effect was selected or committed.
type StrictGenesisIngressResolution struct {
	ingress GenesisAuthorityIngress
	source  projecttypeenvselectionauthority.StrictCLIDurableSourceResult
}

func (resolution StrictGenesisIngressResolution) Ingress() GenesisAuthorityIngress {
	return resolution.ingress
}

func (resolution StrictGenesisIngressResolution) Captured() bool {
	_, ok := resolution.source.Captured()
	return ok
}

func (resolution StrictGenesisIngressResolution) Replayed() bool {
	_, ok := resolution.source.Replayed()
	return ok
}

type currentCLIIngressPosture interface {
	currentCLIIngressPostureVariant()
}

type currentCLIIngressExplicitHDecide struct{}

func (currentCLIIngressExplicitHDecide) currentCLIIngressPostureVariant() {}

type currentCLIIngressStrictCaptured struct{}

func (currentCLIIngressStrictCaptured) currentCLIIngressPostureVariant() {}

type currentCLIIngressStrictReplayed struct{}

func (currentCLIIngressStrictReplayed) currentCLIIngressPostureVariant() {}

// CurrentCLIIngressResolution is the closed outer CLI ingress result. Its
// posture says how authority entered this invocation; it is not a head-effect
// result and carries no selection, Work, or commit claim.
type CurrentCLIIngressResolution struct {
	ingress GenesisAuthorityIngress
	posture currentCLIIngressPosture
}

func (resolution CurrentCLIIngressResolution) Ingress() GenesisAuthorityIngress {
	return resolution.ingress
}

func (resolution CurrentCLIIngressResolution) ExplicitHDecide() bool {
	_, ok := resolution.posture.(currentCLIIngressExplicitHDecide)
	return ok
}

func (resolution CurrentCLIIngressResolution) StrictCaptured() bool {
	_, ok := resolution.posture.(currentCLIIngressStrictCaptured)
	return ok
}

func (resolution CurrentCLIIngressResolution) StrictReplayed() bool {
	_, ok := resolution.posture.(currentCLIIngressStrictReplayed)
	return ok
}

// ResolveCurrentCLIIngress observes the config once and composes the matching
// ingress. The default explicit_h_decide path never touches the capturer or a
// terminal. Strict mode durably captures or replays its separate SpeechAct.
// SelectGenesis re-observes config and project binding inside the head-effect
// transaction, so a change between these calls fails closed.
func (service *GenesisService) ResolveCurrentCLIIngress(
	ctx context.Context,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	reviewCarrier authority.ObservableCarrierBinding,
	capturer projecttypeenvselectionauthority.StrictCLISpeechActCapturer,
) (CurrentCLIIngressResolution, error) {
	if ctx == nil {
		return CurrentCLIIngressResolution{},
			sqlitetransaction.ErrContextRequired
	}
	if service == nil || service.database == nil {
		return CurrentCLIIngressResolution{},
			fmt.Errorf("current Genesis CLI ingress service is unavailable")
	}
	config, configCarrier, err := loadCurrentProjectConfigAuthorityCarrier(
		service.projectRoot,
	)
	if err != nil {
		return CurrentCLIIngressResolution{}, err
	}
	mode, err := headSelectionAuthorityMode(config)
	if err != nil {
		return CurrentCLIIngressResolution{}, err
	}
	if mode ==
		projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityModeExplicitHDecide {
		return CurrentCLIIngressResolution{
			ingress: NewDedicatedCLIInvocation(),
			posture: currentCLIIngressExplicitHDecide{},
		}, nil
	}
	if mode !=
		projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityModeStrictCLISpeechAct {
		return CurrentCLIIngressResolution{},
			fmt.Errorf("current project TypeEnv head-selection mode is unsupported")
	}
	strict, err := service.resolveOrCaptureStrictIngress(
		ctx,
		request,
		content,
		stage,
		configCarrier,
		reviewCarrier,
		capturer,
	)
	if err != nil {
		return CurrentCLIIngressResolution{}, err
	}
	if strict.Captured() {
		return CurrentCLIIngressResolution{
			ingress: strict.Ingress(),
			posture: currentCLIIngressStrictCaptured{},
		}, nil
	}
	if strict.Replayed() {
		return CurrentCLIIngressResolution{
			ingress: strict.Ingress(),
			posture: currentCLIIngressStrictReplayed{},
		}, nil
	}
	return CurrentCLIIngressResolution{},
		fmt.Errorf("strict Genesis ingress returned an invalid posture")
}

// ResolveOrCaptureStrictIngress observes the same config carrier and current
// project-profile binding used by SelectGenesis, then durably resolves the
// strict SpeechAct before the separate head effect. SelectGenesis re-observes
// both sources inside its own effect transaction and fails closed on drift.
func (service *GenesisService) ResolveOrCaptureStrictIngress(
	ctx context.Context,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	reviewCarrier authority.ObservableCarrierBinding,
	capturer projecttypeenvselectionauthority.StrictCLISpeechActCapturer,
) (StrictGenesisIngressResolution, error) {
	if ctx == nil {
		return StrictGenesisIngressResolution{},
			sqlitetransaction.ErrContextRequired
	}
	if service == nil || service.database == nil || capturer == nil {
		return StrictGenesisIngressResolution{},
			fmt.Errorf("strict Genesis ingress service is unavailable")
	}
	config, configCarrier, err := loadCurrentProjectConfigAuthorityCarrier(
		service.projectRoot,
	)
	if err != nil {
		return StrictGenesisIngressResolution{}, err
	}
	mode, err := headSelectionAuthorityMode(config)
	if err != nil {
		return StrictGenesisIngressResolution{}, err
	}
	if mode !=
		projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityModeStrictCLISpeechAct {
		return StrictGenesisIngressResolution{}, fmt.Errorf(
			"current project TypeEnv head-selection mode does not permit strict SpeechAct ingress",
		)
	}
	return service.resolveOrCaptureStrictIngress(
		ctx,
		request,
		content,
		stage,
		configCarrier,
		reviewCarrier,
		capturer,
	)
}

func (service *GenesisService) resolveOrCaptureStrictIngress(
	ctx context.Context,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	configCarrier authority.ObservableCarrierBinding,
	reviewCarrier authority.ObservableCarrierBinding,
	capturer projecttypeenvselectionauthority.StrictCLISpeechActCapturer,
) (StrictGenesisIngressResolution, error) {
	if capturer == nil {
		return StrictGenesisIngressResolution{},
			fmt.Errorf("strict Genesis ingress requires a SpeechAct capturer")
	}
	projectBinding, err := service.loadStrictIngressProjectBinding(
		ctx,
		request,
		content,
	)
	if err != nil {
		return StrictGenesisIngressResolution{}, err
	}
	preparation, err :=
		projecttypeenvselectionauthority.PrepareStrictCLISpeechAct(
			projecttypeenvselectionauthority.StrictCLISpeechActPreparationInput{
				Request:        request,
				Content:        content,
				Stage:          stage,
				ProjectBinding: projectBinding,
				ConfigCarrier:  configCarrier,
				ReviewCarrier:  reviewCarrier,
			},
		)
	if err != nil {
		return StrictGenesisIngressResolution{}, err
	}
	store, err :=
		projecttypeenvselectionauthority.OpenStrictCLISpeechActSourceStore(
			service.database,
			capturer,
		)
	if err != nil {
		return StrictGenesisIngressResolution{}, err
	}
	source, err := store.ResolveOrCapture(ctx, preparation)
	if err != nil {
		return StrictGenesisIngressResolution{}, err
	}
	record, ok := source.Record()
	if !ok {
		return StrictGenesisIngressResolution{},
			fmt.Errorf("strict Genesis source produced no SpeechAct record")
	}
	ingress, err := NewVerifiedSpeechActIngress(
		preparation.ResolverPolicy(),
		record,
	)
	if err != nil {
		return StrictGenesisIngressResolution{}, err
	}
	return StrictGenesisIngressResolution{
		ingress: ingress,
		source:  source,
	}, nil
}

func (service *GenesisService) loadStrictIngressProjectBinding(
	ctx context.Context,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
) (
	projecttypeenvselectionauthority.ProjectAuthorityContextBinding,
	error,
) {
	transaction, err := sqlitetransaction.BeginRead(ctx, service.database)
	if err != nil {
		return projecttypeenvselectionauthority.ProjectAuthorityContextBinding{},
			err
	}
	root, err := loadBoundProjectRootTx(
		ctx,
		transaction,
		request.Project().String(),
	)
	if err != nil {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return projecttypeenvselectionauthority.ProjectAuthorityContextBinding{},
			errors.Join(err, finish.Err())
	}
	if root != service.projectRoot {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return projecttypeenvselectionauthority.ProjectAuthorityContextBinding{},
			errors.Join(
				fmt.Errorf(
					"strict Genesis ingress project root differs from the service root",
				),
				finish.Err(),
			)
	}
	profile, err := loadCurrentProjectProfileBasisTx(ctx, transaction, root)
	if err != nil {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return projecttypeenvselectionauthority.ProjectAuthorityContextBinding{},
			errors.Join(err, finish.Err())
	}
	binding, err := projectAuthorityContextBindingFromCurrentProfile(
		root,
		profile,
		request,
		content,
	)
	if err != nil {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return projecttypeenvselectionauthority.ProjectAuthorityContextBinding{},
			errors.Join(err, finish.Err())
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return projecttypeenvselectionauthority.ProjectAuthorityContextBinding{},
			finish.Err()
	}
	return binding, nil
}
