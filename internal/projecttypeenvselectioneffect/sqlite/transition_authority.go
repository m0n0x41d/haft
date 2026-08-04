package sqlite

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

func resolveCurrentTransitionAuthority(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	root projectprofile.ProjectRootV1,
	clock typedmemorystore.Clock,
	frame currentTransitionFrame,
	input TransitionSelectionInput,
) (currentGenesisAuthorityResult, error) {
	switch ingress := input.Authority.(type) {
	case GenesisAuthorityIngress:
		return resolveCurrentHeadSelectionAuthority(
			ctx,
			transaction,
			root,
			clock,
			currentHeadSelectionAuthorityInput{
				request:   input.Request,
				content:   input.Content,
				authority: ingress,
				profile:   frame.currentProfile,
			},
		)
	case AutomaticCompatibleSuccessorIngress:
		return resolveAutomaticCompatibleSuccessor(
			ctx,
			transaction,
			root,
			clock,
			frame,
			input,
		)
	default:
		return nil, fmt.Errorf(
			"current Transition authority ingress is unsupported: %T",
			input.Authority,
		)
	}
}

func resolveAutomaticCompatibleSuccessor(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	root projectprofile.ProjectRootV1,
	clock typedmemorystore.Clock,
	frame currentTransitionFrame,
	input TransitionSelectionInput,
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
		return nil, fmt.Errorf("automatic compatible-successor clock is required")
	}
	if !frame.currentStage.Valid() {
		return rejectCurrentGenesisAuthorityFor(
			projecttypeenvselectioneffect.NotSelectedStageDrift(),
		), nil
	}
	stage, present := frame.currentStage.Stage()
	if !present {
		return rejectCurrentGenesisAuthorityFor(
			projecttypeenvselectioneffect.NotSelectedStageDrift(),
		), nil
	}
	binding, err := projectAuthorityContextBindingFromCurrentProfile(
		root,
		frame.currentProfile,
		input.Request,
		input.Content,
	)
	if err != nil {
		return nil, err
	}
	evaluatedAt := clock.Now().Round(0).UTC()
	resolution, err :=
		projecttypeenvselectionauthority.SealCompatibleSuccessorResolution(
			projecttypeenvselectionauthority.CompatibleSuccessorResolutionInput{
				Request:        input.Request,
				Content:        input.Content,
				Stage:          stage,
				ProjectBinding: binding,
				EvaluatedAt:    evaluatedAt,
			},
		)
	if err != nil {
		return rejectCurrentGenesisAuthorityFor(
			projecttypeenvselectioneffect.NotSelectedCurrentAuthorityRejection(),
		), nil
	}
	subject, err :=
		projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionPermissionSubject(
			input.Content,
		)
	if err != nil {
		return nil, err
	}
	coordinates, err :=
		projecttypeenvselectioneffect.NewCompatibleSuccessorPolicyAuthorityCoordinates(
			projecttypeenvselectioneffect.CompatibleSuccessorPolicyAuthorityCoordinatesInput{
				ContentRef:           input.Content.DescriptionRef(),
				ContentDigest:        input.Content.Digest(),
				ExecutionSubject:     subject,
				ProjectBindingDigest: binding.Digest(),
				PolicyDigest:         resolution.PolicyDigest(),
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
			compatibleSuccessorResolution: resolution,
			content:                       input.Content,
			coordinates:                   coordinates,
		},
	}
	return currentGenesisAuthorityReady{use: use}, nil
}
