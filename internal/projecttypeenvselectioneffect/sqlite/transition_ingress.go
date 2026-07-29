package sqlite

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// ResolveCurrentCLIIngress resolves the configured CLI authority source for
// one exact post-Genesis transition. The authority machinery is shared with
// Genesis, but this boundary refuses a Genesis request before observing or
// persisting any strict source.
func (service *TransitionService) ResolveCurrentCLIIngress(
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
	if service == nil || service.core == nil {
		return CurrentCLIIngressResolution{},
			fmt.Errorf("current Transition CLI ingress service is unavailable")
	}
	predecessorValue := request.Predecessor()
	predecessor, ok := predecessorValue.(projecttypeenvselection.TransitionStagePredecessor)
	if !ok {
		return CurrentCLIIngressResolution{},
			fmt.Errorf("transition CLI ingress requires an exact prior head")
	}
	prior, err := projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           predecessor.Project(),
			SelectedComposite: predecessor.SelectedComposite(),
			Revision:          predecessor.HeadRevision(),
		},
	)
	if err != nil {
		return CurrentCLIIngressResolution{},
			fmt.Errorf("restore Transition CLI ingress prior head: %w", err)
	}
	if err := projecttypeenvselection.
		VerifyTransitionProjectTypeEnvHeadSelectionRequestStructure(
			request,
			prior,
			stage,
		); err != nil {
		return CurrentCLIIngressResolution{},
			fmt.Errorf("verify Transition CLI ingress request: %w", err)
	}
	if err := content.ExactAgainst(request); err != nil {
		return CurrentCLIIngressResolution{},
			fmt.Errorf("verify Transition CLI ingress content: %w", err)
	}
	return service.core.ResolveCurrentCLIIngress(
		ctx,
		request,
		content,
		stage,
		reviewCarrier,
		capturer,
	)
}
