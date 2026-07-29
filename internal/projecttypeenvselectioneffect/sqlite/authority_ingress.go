package sqlite

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type genesisAuthorityIngressVariant interface {
	genesisAuthorityIngressVariant()
}

// DedicatedCLIInvocation marks the lower-assurance dedicated CLI ingress. It
// carries no caller-made authority resolution. The service seals the exact
// source and resolution only after replay absence and current config reads.
type DedicatedCLIInvocation struct{}

func (DedicatedCLIInvocation) genesisAuthorityIngressVariant() {}

// VerifiedSpeechActIngress carries only the already verified human source
// record plus the resolver-policy edition needed to interpret it. The service
// still rebuilds the strict mode policy from the current config and resolves
// authority against the current transaction Stage.
type VerifiedSpeechActIngress struct {
	resolverPolicy projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionResolverPolicy
	record         projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionSpeechActRecord
}

func (VerifiedSpeechActIngress) genesisAuthorityIngressVariant() {}

// GenesisAuthorityIngress is a closed source union. It deliberately cannot
// hold an AuthorityResolutionRecord or a live authority-use capability.
type GenesisAuthorityIngress struct {
	variant genesisAuthorityIngressVariant
}

func NewDedicatedCLIInvocation() GenesisAuthorityIngress {
	return GenesisAuthorityIngress{variant: DedicatedCLIInvocation{}}
}

func NewVerifiedSpeechActIngress(
	resolverPolicy projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionResolverPolicy,
	record projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionSpeechActRecord,
) (GenesisAuthorityIngress, error) {
	if err := resolverPolicy.ExactAgainst(
		resolverPolicy.SourceContract(),
		resolverPolicy.SourceAdapter(),
		resolverPolicy.ProjectBinding(),
	); err != nil {
		return GenesisAuthorityIngress{}, fmt.Errorf(
			"verified SpeechAct ingress resolver policy: %w",
			err,
		)
	}
	if err := record.Verify(record.Content().Request()); err != nil {
		return GenesisAuthorityIngress{}, fmt.Errorf(
			"verified SpeechAct ingress record: %w",
			err,
		)
	}
	return GenesisAuthorityIngress{
		variant: VerifiedSpeechActIngress{
			resolverPolicy: resolverPolicy,
			record:         record,
		},
	}, nil
}

type resolvedGenesisAuthority struct {
	policy      projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityPolicyRecord
	source      projecttypeenvselectionauthority.AuthoritySourceRecord
	resolution  projecttypeenvselectionauthority.AuthorityResolutionRecord
	coordinates projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionAuthorityCoordinates
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
			stage:     frame.readyStage.Stage(),
			profile:   frame.currentProfile,
		},
	)
}
