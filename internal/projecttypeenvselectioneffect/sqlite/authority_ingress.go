package sqlite

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// GenesisAuthorityIngress carries one exact host-routed operator request. It
// cannot contain a caller-made resolution or a live authority-use capability.
type GenesisAuthorityIngress struct {
	request operatorrequest.Request
}

func NewHostRoutedOperatorRequest(
	request operatorrequest.Request,
) (GenesisAuthorityIngress, error) {
	restored, err := operatorrequest.FromCoordinates(
		request.Effect(),
		request.SubjectRef(),
		request.PayloadDigest(),
		request.Digest(),
	)
	if err != nil || restored != request ||
		request.Provenance() != operatorrequest.HostRoutedOperatorRequest ||
		request.Effect() != operatorrequest.ProjectTypeEnvHeadSelect {
		return GenesisAuthorityIngress{}, fmt.Errorf(
			"TypeEnv authority ingress requires an exact host-routed operator request",
		)
	}
	return GenesisAuthorityIngress{request: request}, nil
}

func (ingress GenesisAuthorityIngress) Request() operatorrequest.Request {
	return ingress.request
}

type resolvedGenesisAuthority struct {
	request        operatorrequest.Request
	hostResolution projecttypeenvselectionauthority.HostRoutedSelectionResolution
	content        projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent
	coordinates    projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionAuthorityCoordinates
}

// admittedGenesisAuthorityUse is minted and consumed inside one transaction.
// It has no codec, no exported constructor, and no value-copy consumption API.
type admittedGenesisAuthorityUse struct {
	transaction *sqlitetransaction.Transaction
	resolved    resolvedGenesisAuthority
	consumed    bool
}

func (use *admittedGenesisAuthorityUse) consume(
	transaction *sqlitetransaction.Transaction,
) (resolvedGenesisAuthority, error) {
	if use == nil || use.transaction == nil || transaction == nil {
		return resolvedGenesisAuthority{}, fmt.Errorf(
			"genesis authority use is invalid",
		)
	}
	if use.transaction != transaction {
		return resolvedGenesisAuthority{}, fmt.Errorf(
			"genesis authority use belongs to another transaction",
		)
	}
	if err := transaction.RequireImmediate(); err != nil {
		return resolvedGenesisAuthority{}, err
	}
	if use.consumed {
		return resolvedGenesisAuthority{}, fmt.Errorf(
			"genesis authority use was already consumed",
		)
	}
	use.consumed = true
	return use.resolved, nil
}

func (service *GenesisService) resolveCurrentAuthority(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	frame currentGenesisFrame,
	input GenesisSelectionInput,
) (currentGenesisAuthorityResult, error) {
	return resolveCurrentHeadSelectionAuthority(
		ctx,
		transaction,
		service.projectRoot,
		service.clock,
		currentHeadSelectionAuthorityInput{
			request:   input.Request,
			content:   input.Content,
			authority: input.Authority,
			profile:   frame.currentProfile,
		},
	)
}
