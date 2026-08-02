package sqlite

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type currentHeadSelectionAuthorityInput struct {
	request   projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	content   projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent
	authority GenesisAuthorityIngress
	profile   projecttypeenvprofilebasis.CurrentProjectProfileBasis
}

func resolveCurrentHeadSelectionAuthority(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	root projectprofile.ProjectRootV1,
	clock typedmemorystore.Clock,
	input currentHeadSelectionAuthorityInput,
) (currentGenesisAuthorityResult, error) {
	if ctx == nil {
		return nil, sqlitetransaction.ErrContextRequired
	}
	if transaction == nil {
		return nil, sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireImmediate(); err != nil {
		return nil, err
	}
	if clock == nil {
		return nil, fmt.Errorf("current Genesis authority clock is required")
	}
	evaluatedAt := clock.Now().Round(0).UTC()
	if evaluatedAt.IsZero() {
		return nil, fmt.Errorf("current Genesis authority time is required")
	}
	if !input.content.ValidityWindow().Contains(evaluatedAt) {
		return rejectCurrentGenesisAuthorityFor(
			projecttypeenvselectioneffect.NotSelectedReviewExpired(),
		), nil
	}
	payload, err := projecttypeenvselectionauthority.HostRoutedSelectionPayload(
		input.request,
		input.content,
	)
	if err != nil {
		return nil, err
	}
	request := input.authority.Request()
	if request.Provenance() != operatorrequest.HostRoutedOperatorRequest ||
		request.Effect() != operatorrequest.ProjectTypeEnvHeadSelect ||
		request.SubjectRef() != input.request.Ref().String() ||
		!request.MatchesPayload(payload) {
		return rejectCurrentGenesisAuthority(), nil
	}
	projectBinding, err := projectAuthorityContextBindingFromCurrentProfile(
		root,
		input.profile,
		input.request,
		input.content,
	)
	if err != nil {
		return nil, err
	}
	resolution, err :=
		projecttypeenvselectionauthority.SealHostRoutedSelectionResolution(
			projecttypeenvselectionauthority.HostRoutedSelectionResolutionInput{
				OperatorRequest:  request,
				SelectionRequest: input.request,
				Content:          input.content,
				ProjectBinding:   projectBinding,
				EvaluatedAt:      evaluatedAt,
			},
		)
	if err != nil {
		return rejectCurrentGenesisAuthority(), nil
	}
	subject, err :=
		projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionPermissionSubject(
			input.content,
		)
	if err != nil {
		return nil, err
	}
	coordinates, err :=
		projecttypeenvselectioneffect.NewHostRoutedOperatorRequestAuthorityCoordinates(
			projecttypeenvselectioneffect.HostRoutedOperatorRequestAuthorityCoordinatesInput{
				ContentRef:           input.content.DescriptionRef(),
				ContentDigest:        input.content.Digest(),
				ExecutionSubject:     subject,
				OperatorRequest:      request,
				ProjectBindingDigest: projectBinding.Digest(),
				ResolutionRef:        resolution.Ref(),
				ResolutionDigest:     resolution.Digest(),
			},
		)
	if err != nil {
		return nil, err
	}
	use := &admittedGenesisAuthorityUse{
		transaction: transaction,
		resolved: resolvedGenesisAuthority{
			request:        request,
			hostResolution: resolution,
			content:        input.content,
			coordinates:    coordinates,
		},
	}
	return currentGenesisAuthorityReady{use: use}, nil
}

func projectAuthorityContextBindingFromCurrentProfile(
	projectRoot projectprofile.ProjectRootV1,
	currentProfile projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
) (projecttypeenvselectionauthority.ProjectAuthorityContextBinding, error) {
	root, err := authority.NewProjectRoot(projectRoot.String())
	if err != nil {
		return projecttypeenvselectionauthority.ProjectAuthorityContextBinding{},
			fmt.Errorf("seal current authority project root: %w", err)
	}
	carrierRef, err := authority.NewCarrierRef(
		currentProfile.ProfileBasisRef().String(),
	)
	if err != nil {
		return projecttypeenvselectionauthority.ProjectAuthorityContextBinding{},
			fmt.Errorf("seal current profile carrier ref: %w", err)
	}
	carrierDigest, err := authority.NewDigest(
		currentProfile.Digest().String(),
	)
	if err != nil {
		return projecttypeenvselectionauthority.ProjectAuthorityContextBinding{},
			fmt.Errorf("seal current profile carrier digest: %w", err)
	}
	carrier, err := authority.NewObservableCarrierBinding(
		carrierRef,
		carrierDigest,
	)
	if err != nil {
		return projecttypeenvselectionauthority.ProjectAuthorityContextBinding{},
			fmt.Errorf("seal current profile carrier: %w", err)
	}
	binding, err :=
		projecttypeenvselectionauthority.SealProjectAuthorityContextBinding(
			projecttypeenvselectionauthority.ProjectAuthorityContextBindingInput{
				Project: request.Project(),
				Root:    root,
				Context: content.JudgementContext(),
				Carrier: carrier,
			},
		)
	if err != nil {
		return projecttypeenvselectionauthority.ProjectAuthorityContextBinding{},
			fmt.Errorf("seal current project authority binding: %w", err)
	}
	return binding, nil
}
