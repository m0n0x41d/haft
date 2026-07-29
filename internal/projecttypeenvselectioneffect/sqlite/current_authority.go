package sqlite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

const (
	projectConfigCarrierRef        = "carrier:.haft/config.yaml"
	defaultProjectConfigCarrierRef = "carrier:built-in/project-config-default/v1"
)

type currentHeadSelectionAuthorityInput struct {
	request   projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	content   projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent
	authority GenesisAuthorityIngress
	stage     projecttypeenvselection.ProjectTypeEnvStage
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
	config, carrier, err := loadCurrentProjectConfigAuthorityCarrier(root)
	if err != nil {
		return nil, err
	}
	mode, err := headSelectionAuthorityMode(config)
	if err != nil {
		return nil, err
	}
	configBasis, err :=
		projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionConfigAuthorityBasis(
			input.request.Project(),
			mode,
			carrier,
		)
	if err != nil {
		return nil, fmt.Errorf("seal current config authority basis: %w", err)
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
	evaluatedAt := clock.Now().Round(0).UTC()
	if evaluatedAt.IsZero() {
		return nil, fmt.Errorf("current Genesis authority time is required")
	}
	switch ingress := input.authority.variant.(type) {
	case DedicatedCLIInvocation:
		return resolveDedicatedCLIAuthority(
			transaction,
			configBasis,
			projectBinding,
			evaluatedAt,
			input.request,
			input.content,
		)
	case VerifiedSpeechActIngress:
		return resolveStrictSpeechActAuthority(
			transaction,
			configBasis,
			projectBinding,
			evaluatedAt,
			input.stage,
			input.request,
			input.content,
			ingress,
		)
	default:
		return nil, fmt.Errorf("genesis authority ingress variant is required")
	}
}

func resolveDedicatedCLIAuthority(
	transaction *sqlitetransaction.Transaction,
	configBasis projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionConfigAuthorityBasis,
	projectBinding projecttypeenvselectionauthority.ProjectAuthorityContextBinding,
	evaluatedAt time.Time,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
) (currentGenesisAuthorityResult, error) {
	if configBasis.Mode() !=
		projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityModeExplicitHDecide {
		return rejectCurrentGenesisAuthority(), nil
	}
	if !content.ValidityWindow().Contains(evaluatedAt) {
		return rejectCurrentGenesisAuthorityFor(
			projecttypeenvselectioneffect.NotSelectedReviewExpired(),
		), nil
	}
	policy, err :=
		projecttypeenvselectionauthority.SealExplicitHDecideAuthorityPolicy(
			configBasis,
			projectBinding,
		)
	if err != nil {
		return nil, err
	}
	policyRecord, err :=
		projecttypeenvselectionauthority.NewAuthorityPolicyFromExplicitHDecide(
			policy,
		)
	if err != nil {
		return nil, err
	}
	source, err :=
		projecttypeenvselectionauthority.SealTrustedDedicatedCLIInvocationSourceRecord(
			projecttypeenvselectionauthority.TrustedDedicatedCLIInvocationSourceRecordInput{
				Policy:     policy,
				Request:    request,
				Content:    content,
				RecordedAt: evaluatedAt,
			},
		)
	if err != nil {
		return nil, err
	}
	sourceRecord, err :=
		projecttypeenvselectionauthority.NewAuthoritySourceFromTrustedDedicatedCLIInvocation(
			source,
		)
	if err != nil {
		return nil, err
	}
	resolution, err :=
		projecttypeenvselectionauthority.SealExplicitPolicyAcceptanceResolution(
			projecttypeenvselectionauthority.ExplicitPolicyAcceptanceResolutionInput{
				Source:      source,
				EvaluatedAt: evaluatedAt,
			},
		)
	if err != nil {
		return nil, err
	}
	resolutionRecord, err :=
		projecttypeenvselectionauthority.NewAuthorityResolutionFromExplicitPolicyAcceptance(
			resolution,
		)
	if err != nil {
		return nil, err
	}
	use, err := mintAdmittedGenesisAuthorityUse(
		transaction,
		policyRecord,
		sourceRecord,
		resolutionRecord,
	)
	if err != nil {
		return nil, err
	}
	return currentGenesisAuthorityReady{use: use}, nil
}

func resolveStrictSpeechActAuthority(
	transaction *sqlitetransaction.Transaction,
	configBasis projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionConfigAuthorityBasis,
	projectBinding projecttypeenvselectionauthority.ProjectAuthorityContextBinding,
	evaluatedAt time.Time,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
	ingress VerifiedSpeechActIngress,
) (currentGenesisAuthorityResult, error) {
	if configBasis.Mode() !=
		projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityModeStrictCLISpeechAct {
		return rejectCurrentGenesisAuthority(), nil
	}
	resolverBinding := ingress.resolverPolicy.ProjectBinding()
	if resolverBinding.Digest() != projectBinding.Digest() ||
		!bytes.Equal(
			resolverBinding.CanonicalJSON(),
			projectBinding.CanonicalJSON(),
		) {
		return rejectCurrentGenesisAuthority(), nil
	}
	policy, err :=
		projecttypeenvselectionauthority.SealStrictCLISpeechActAuthorityPolicy(
			configBasis,
			ingress.resolverPolicy,
		)
	if err != nil {
		return nil, err
	}
	policyRecord, err :=
		projecttypeenvselectionauthority.NewAuthorityPolicyFromStrictCLISpeechAct(
			policy,
		)
	if err != nil {
		return nil, err
	}
	if err := ingress.record.Verify(request); err != nil {
		return nil, fmt.Errorf("verify strict SpeechAct record: %w", err)
	}
	if ingress.record.Content().Digest() != content.Digest() {
		return nil, fmt.Errorf(
			"strict SpeechAct record names another authorization content",
		)
	}
	basis, err :=
		projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionAuthorityResolutionBasis(
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput{
				Policy:      ingress.resolverPolicy,
				Record:      ingress.record,
				Content:     content,
				Request:     request,
				Stage:       stage,
				EvaluatedAt: evaluatedAt,
			},
		)
	if err != nil {
		if currentAuthorityResolutionRejected(err) {
			return rejectCurrentGenesisAuthority(), nil
		}
		return nil, err
	}
	resolution, err :=
		projecttypeenvselectionauthority.SealStrictPermissionResolution(basis)
	if err != nil {
		return nil, err
	}
	source, err :=
		projecttypeenvselectionauthority.SealVerifiedSpeechActAuthoritySourceRecord(
			ingress.record,
		)
	if err != nil {
		return nil, err
	}
	sourceRecord, err :=
		projecttypeenvselectionauthority.NewAuthoritySourceFromVerifiedSpeechAct(
			source,
		)
	if err != nil {
		return nil, err
	}
	resolutionRecord, err :=
		projecttypeenvselectionauthority.NewAuthorityResolutionFromStrictPermission(
			resolution,
		)
	if err != nil {
		return nil, err
	}
	use, err := mintAdmittedGenesisAuthorityUse(
		transaction,
		policyRecord,
		sourceRecord,
		resolutionRecord,
	)
	if err != nil {
		return nil, err
	}
	return currentGenesisAuthorityReady{use: use}, nil
}

func currentAuthorityResolutionRejected(err error) bool {
	rejection := projecttypeenvselectionauthority.AuthorityRejection{}
	if !errors.As(err, &rejection) {
		return false
	}
	return rejection.Code() !=
		projecttypeenvselectionauthority.AuthorityRejectedInvalidBasis
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

func mintAdmittedGenesisAuthorityUse(
	transaction *sqlitetransaction.Transaction,
	policy projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityPolicyRecord,
	source projecttypeenvselectionauthority.AuthoritySourceRecord,
	resolution projecttypeenvselectionauthority.AuthorityResolutionRecord,
) (*admittedGenesisAuthorityUse, error) {
	coordinates, err :=
		projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionAuthorityCoordinatesFromResolution(
			policy,
			resolution,
		)
	if err != nil {
		return nil, err
	}
	return &admittedGenesisAuthorityUse{
		transaction: transaction,
		resolved: resolvedGenesisAuthority{
			policy:      policy,
			source:      source,
			resolution:  resolution,
			coordinates: coordinates,
		},
	}, nil
}

func loadCurrentProjectConfigAuthorityCarrier(
	root projectprofile.ProjectRootV1,
) (
	project.ProjectConfig,
	authority.ObservableCarrierBinding,
	error,
) {
	path := project.ProjectConfigPath(filepath.Join(root.String(), ".haft"))
	bytes, err := os.ReadFile(path)
	carrierRef := projectConfigCarrierRef
	if errors.Is(err, os.ErrNotExist) {
		bytes = []byte(project.ExampleProjectConfigYAML())
		carrierRef = defaultProjectConfigCarrierRef
		err = nil
	}
	if err != nil {
		return project.ProjectConfig{}, authority.ObservableCarrierBinding{},
			fmt.Errorf("read current project config %s: %w", path, err)
	}
	config, err := project.ParseProjectConfig(bytes)
	if err != nil {
		return project.ProjectConfig{}, authority.ObservableCarrierBinding{},
			fmt.Errorf("parse current project config %s: %w", path, err)
	}
	ref, err := authority.NewCarrierRef(carrierRef)
	if err != nil {
		return project.ProjectConfig{}, authority.ObservableCarrierBinding{}, err
	}
	sum := sha256.Sum256(bytes)
	digest, err := authority.NewDigest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	if err != nil {
		return project.ProjectConfig{}, authority.ObservableCarrierBinding{}, err
	}
	carrier, err := authority.NewObservableCarrierBinding(ref, digest)
	if err != nil {
		return project.ProjectConfig{}, authority.ObservableCarrierBinding{}, err
	}
	return config, carrier, nil
}

func headSelectionAuthorityMode(
	config project.ProjectConfig,
) (projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityMode, error) {
	switch config.EffectiveProjectTypeEnvHeadSelectionMode() {
	case project.ProjectTypeEnvHeadSelectionModeExplicitHDecide:
		return projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityModeExplicitHDecide,
			nil
	case project.ProjectTypeEnvHeadSelectionModeStrictCLISpeechAct:
		return projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityModeStrictCLISpeechAct,
			nil
	default:
		return 0, fmt.Errorf("current project TypeEnv head-selection mode is unsupported")
	}
}
