package projecttypeenvselection

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

var (
	stageVerificationOnce sync.Once
	stageVerification     projecttypeenv.ProjectTypeEnvCompositeVerification
	stageVerificationErr  error
)

func mustStageCompositeVerification(
	t *testing.T,
) projecttypeenv.ProjectTypeEnvCompositeVerification {
	t.Helper()
	stageVerificationOnce.Do(func() {
		stageVerification, stageVerificationErr = buildStageCompositeVerification()
	})
	if stageVerificationErr != nil {
		t.Fatalf("build Stage composite verification: %v", stageVerificationErr)
	}
	if err := stageVerification.Verify(); err != nil {
		t.Fatalf("verify Stage composite capability: %v", err)
	}
	return stageVerification
}

func buildStageCompositeVerification() (
	projecttypeenv.ProjectTypeEnvCompositeVerification,
	error,
) {
	databasePath, err := filepath.Abs(filepath.Join("..", "cli", "fpf.db"))
	if err != nil {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{}, err
	}
	database, err := sql.Open(
		"sqlite",
		"file:"+filepath.ToSlash(databasePath)+"?mode=ro&immutable=1",
	)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{}, err
	}
	database.SetMaxOpenConns(1)
	defer func() { _ = database.Close() }()

	base, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{}, err
	}
	resolution := projecttypeenv.LinkProjectTypeEnvCompositeIR(base, nil)
	if resolution.Rejected() {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{}, fmt.Errorf(
			"link empty project extension DAG: %#v",
			resolution.Issues(),
		)
	}
	linked, exists := resolution.CompositeIR()
	if !exists {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{}, fmt.Errorf(
			"accepted empty project extension DAG has no linked IR",
		)
	}
	runtimeBasis, err := stageRuntimeEvaluationBasis(base, linked)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{}, err
	}
	composite, err := projecttypeenv.SealProjectTypeEnvComposite(linked, runtimeBasis)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{}, err
	}
	preparation := projecttypeenv.PrepareProjectTypeEnvComposite(
		projecttypeenv.ProjectTypeEnvCompositePreparationInput{
			Base:         base,
			Linked:       linked,
			RuntimeBasis: runtimeBasis,
			Composite:    composite,
		},
	)
	if preparation.Rejected() {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{}, fmt.Errorf(
			"prepare empty project extension composite: %#v",
			preparation.Issues(),
		)
	}
	verification, exists := preparation.Verification()
	if !exists {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{}, fmt.Errorf(
			"accepted composite preparation has no verification capability",
		)
	}
	return verification, nil
}

func stageRuntimeEvaluationBasis(
	base typeenv.BaseTypeEnvArtifact,
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
) (projecttypeenv.RuntimeEvaluationBasisArtifact, error) {
	emptyBasis, err := projecttypeenv.SealRuntimeEvaluationBasis(nil)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	provisionalComposite, err := projecttypeenv.SealProjectTypeEnvComposite(
		linked,
		emptyBasis,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	candidate, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(
		base,
		provisionalComposite.Ref(),
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	resolution := projecttypeenv.ResolveProjectTypeEnvCompositeRuntimeRequirements(
		provisionalComposite,
		candidate,
		linked,
		emptyBasis,
	)
	requirements := resolution.RequiredSet().Requirements()
	if len(requirements) == 0 {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, fmt.Errorf(
			"provisional composite has no runtime requirements",
		)
	}
	return sealStageRuntimeEvaluationBasis(requirements)
}

func sealStageRuntimeEvaluationBasis(
	requirements []projecttypeenv.CompositeRuntimeRequirement,
) (projecttypeenv.RuntimeEvaluationBasisArtifact, error) {
	entries := make([]runtimemechanism.RuntimeMechanismEntryV1, 0, len(requirements))
	for _, requirement := range requirements {
		entry, err := stageRuntimeMechanismEntry(requirement)
		if err != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
		}
		entries = append(entries, entry)
	}
	artifactRef, err := typedmemory.NewCarrierRef("artifact:stage-witness-runtime")
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	artifact, err := runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifactRef,
		edition,
		entries,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	mechanism, err := projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(artifact)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	pins := make([]projecttypeenv.RuntimeEvaluationMechanismPin, 0, len(requirements))
	for _, requirement := range requirements {
		pin, err := stageRuntimeMechanismPin(requirement, mechanism, artifact)
		if err != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
		}
		pins = append(pins, pin)
	}
	return projecttypeenv.SealRuntimeEvaluationBasis(pins, artifact)
}

func stageRuntimeMechanismEntry(
	requirement projecttypeenv.CompositeRuntimeRequirement,
) (runtimemechanism.RuntimeMechanismEntryV1, error) {
	codec, hasCodec := requirement.Codec()
	if hasCodec {
		return runtimemechanism.NewCodecCanonicalizationEntry(codec)
	}
	rule, hasRule := requirement.Rule()
	if !hasRule {
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf(
			"runtime requirement %q has no semantic reference",
			requirement.SemanticReference(),
		)
	}
	switch requirement.InvocationContract() {
	case projecttypeenv.RuntimeMechanismContractEntitySetEnumeration:
		return runtimemechanism.NewEntitySetEnumerationEntry(rule)
	case projecttypeenv.RuntimeMechanismContractCandidateVisibility:
		return runtimemechanism.NewCandidateVisibilityEntry(rule)
	case projecttypeenv.RuntimeMechanismContractKindDefinedness:
		return runtimemechanism.NewKindDefinednessEntry(rule)
	case projecttypeenv.RuntimeMechanismContractMemberOf:
		return runtimemechanism.NewMemberOfEntry(rule)
	case projecttypeenv.RuntimeMechanismContractCarrierMembershipDelivery:
		return runtimemechanism.NewCarrierMembershipDeliveryEntry(rule)
	default:
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf(
			"unsupported runtime invocation contract %q",
			requirement.InvocationContract(),
		)
	}
}

func stageRuntimeMechanismPin(
	requirement projecttypeenv.CompositeRuntimeRequirement,
	mechanism projecttypeenv.RuntimeMechanismArtifactPin,
	artifact runtimemechanism.RuntimeMechanismArtifactV1,
) (projecttypeenv.RuntimeEvaluationMechanismPin, error) {
	codec, hasCodec := requirement.Codec()
	if hasCodec {
		return projecttypeenv.NewCodecRuntimeMechanismPin(
			projecttypeenv.CodecRuntimeMechanismPinInput{
				Codec:            codec,
				Mechanism:        mechanism,
				ResolvedArtifact: &artifact,
			},
		)
	}
	rule, hasRule := requirement.Rule()
	if !hasRule {
		return nil, fmt.Errorf(
			"runtime requirement %q has no semantic reference",
			requirement.SemanticReference(),
		)
	}
	if requirement.Role() == projecttypeenv.RuntimeMechanismRoleCarrierMembership {
		return projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
			projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
				Rule:             rule,
				Mechanism:        mechanism,
				ResolvedArtifact: &artifact,
			},
		)
	}
	return projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         requirement.InvocationContract(),
			Mechanism:        mechanism,
			ResolvedArtifact: &artifact,
		},
	)
}
