package projecttypeenvstore

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
)

// ArtifactClosure is a verified immutable B/E/X/C artifact closure. It proves
// exact recipe identity and resolution of every X closure family, but not
// successful final lowering into an executable TypeEnv. It is not a Stage,
// does not select a project head, and carries no authorization.
type ArtifactClosure struct {
	base             typeenv.BaseTypeEnvArtifact
	extensions       []projecttypeenv.ProjectTypeEnvExtensionArtifact
	runtime          projecttypeenv.RuntimeEvaluationBasisArtifact
	mechanisms       []runtimemechanism.RuntimeMechanismArtifactV1
	policies         []projecttypeenv.RegistrationPolicyArtifact
	composite        projecttypeenv.ProjectTypeEnvCompositeArtifact
	records          []artifactRecord
	mechanismRecords []runtimeMechanismRecord
	policyRecords    []registrationPolicyRecord
}

// PrepareArtifactClosure is the pure boundary that proves the exact B,
// canonical E DAG, X, and C recipe before any SQL effect occurs.
func PrepareArtifactClosure(
	base typeenv.BaseTypeEnvArtifact,
	extensions []projecttypeenv.ProjectTypeEnvExtensionArtifact,
	runtime projecttypeenv.RuntimeEvaluationBasisArtifact,
	composite projecttypeenv.ProjectTypeEnvCompositeArtifact,
) (ArtifactClosure, error) {
	if len(runtime.AllPins()) != 0 {
		return ArtifactClosure{}, fmt.Errorf(
			"%w: non-empty X %q must be prepared with PrepareArtifactClosureWithRuntimeClosure",
			ErrRuntimeClosureRequired,
			runtime.Ref().String(),
		)
	}
	return PrepareArtifactClosureWithRuntimeMechanisms(
		base,
		extensions,
		runtime,
		composite,
		nil,
	)
}

// PrepareArtifactClosureWithRuntimeMechanisms additionally binds every
// non-empty X pin to the exact immutable RuntimeMechanismArtifactV1 bytes that
// must be persisted for transitive verification after reread.
func PrepareArtifactClosureWithRuntimeMechanisms(
	base typeenv.BaseTypeEnvArtifact,
	extensions []projecttypeenv.ProjectTypeEnvExtensionArtifact,
	runtime projecttypeenv.RuntimeEvaluationBasisArtifact,
	composite projecttypeenv.ProjectTypeEnvCompositeArtifact,
	mechanisms []runtimemechanism.RuntimeMechanismArtifactV1,
) (ArtifactClosure, error) {
	return PrepareArtifactClosureWithRuntimeClosure(
		base,
		extensions,
		runtime,
		composite,
		mechanisms,
		nil,
	)
}

// PrepareArtifactClosureWithRuntimeClosure binds both exact X closure families
// before any SQL effect occurs.
func PrepareArtifactClosureWithRuntimeClosure(
	base typeenv.BaseTypeEnvArtifact,
	extensions []projecttypeenv.ProjectTypeEnvExtensionArtifact,
	runtime projecttypeenv.RuntimeEvaluationBasisArtifact,
	composite projecttypeenv.ProjectTypeEnvCompositeArtifact,
	mechanisms []runtimemechanism.RuntimeMechanismArtifactV1,
	policies []projecttypeenv.RegistrationPolicyArtifact,
) (ArtifactClosure, error) {
	baseRecord, verifiedBase, err := prepareBaseArtifact(base)
	if err != nil {
		return ArtifactClosure{}, err
	}
	extensionRecords := make([]artifactRecord, 0, len(extensions))
	verifiedExtensions := make([]projecttypeenv.ProjectTypeEnvExtensionArtifact, 0, len(extensions))
	for index, extension := range extensions {
		record, verified, extensionErr := prepareExtensionArtifact(extension)
		if extensionErr != nil {
			return ArtifactClosure{}, fmt.Errorf("prepare E[%d]: %w", index, extensionErr)
		}
		extensionRecords = append(extensionRecords, record)
		verifiedExtensions = append(verifiedExtensions, verified)
	}
	runtimeRecord, verifiedRuntime, err := prepareRuntimeBasisArtifact(runtime)
	if err != nil {
		return ArtifactClosure{}, err
	}
	mechanismRecords, verifiedMechanisms, err := prepareRuntimeMechanismArtifacts(mechanisms)
	if err != nil {
		return ArtifactClosure{}, err
	}
	policyRecords, verifiedPolicies, err := prepareRegistrationPolicyArtifacts(policies)
	if err != nil {
		return ArtifactClosure{}, err
	}
	verifiedRuntime, err = projecttypeenv.ResolveRuntimeEvaluationBasisClosure(
		verifiedRuntime,
		verifiedMechanisms,
		verifiedPolicies,
	)
	if err != nil {
		return ArtifactClosure{}, fmt.Errorf(
			"%w: resolve exact X closure: %v",
			ErrRuntimeClosureRequired,
			err,
		)
	}
	if err := verifiedRuntime.VerifyResolvedClosure(); err != nil {
		return ArtifactClosure{}, fmt.Errorf(
			"%w: X does not carry its exact resolved closure: %v",
			ErrClosureInconsistent,
			err,
		)
	}
	compositeRecord, verifiedComposite, err := prepareCompositeArtifact(composite)
	if err != nil {
		return ArtifactClosure{}, err
	}

	resolution := projecttypeenv.LinkProjectTypeEnvCompositeIR(
		verifiedBase,
		verifiedExtensions,
	)
	if resolution.Rejected() {
		return ArtifactClosure{}, closureLinkError(resolution.Issues())
	}
	linked, exists := resolution.CompositeIR()
	if !exists {
		return ArtifactClosure{}, fmt.Errorf(
			"%w: accepted B/E link produced no composite IR",
			ErrClosureInconsistent,
		)
	}
	expectedComposite, err := resealArtifactClosureComposite(
		linked,
		verifiedRuntime,
		verifiedComposite,
	)
	if err != nil {
		return ArtifactClosure{}, fmt.Errorf(
			"%w: derive C from supplied B/E/X: %v",
			ErrClosureInconsistent,
			err,
		)
	}
	if expectedComposite.Ref() != verifiedComposite.Ref() ||
		!bytes.Equal(expectedComposite.CanonicalBytes(), verifiedComposite.CanonicalBytes()) {
		return ArtifactClosure{}, fmt.Errorf(
			"%w: supplied C %q does not equal C %q derived from exact B/E/X",
			ErrClosureInconsistent,
			verifiedComposite.Ref().String(),
			expectedComposite.Ref().String(),
		)
	}
	orderedExtensions := make([]projecttypeenv.ProjectTypeEnvExtensionArtifact, 0)
	orderedExtensionRecords := make([]artifactRecord, 0)
	recordsByRef := make(map[string]artifactRecord, len(extensionRecords))
	for _, record := range extensionRecords {
		recordsByRef[record.ref] = record
	}
	for _, extension := range linked.Extensions() {
		artifact := extension.Artifact()
		record, exists := recordsByRef[artifact.Ref().String()]
		if !exists {
			return ArtifactClosure{}, fmt.Errorf(
				"%w: linked E %q has no verified storage record",
				ErrClosureInconsistent,
				artifact.Ref().String(),
			)
		}
		orderedExtensions = append(orderedExtensions, artifact)
		orderedExtensionRecords = append(orderedExtensionRecords, record)
		delete(recordsByRef, artifact.Ref().String())
	}
	if len(recordsByRef) != 0 {
		return ArtifactClosure{}, fmt.Errorf(
			"%w: %d supplied E artifact(s) are absent from the canonical linked DAG",
			ErrClosureInconsistent,
			len(recordsByRef),
		)
	}

	records := make([]artifactRecord, 0, len(orderedExtensionRecords)+3)
	records = append(records, baseRecord)
	records = append(records, orderedExtensionRecords...)
	records = append(records, runtimeRecord, compositeRecord)
	return ArtifactClosure{
		base:             verifiedBase,
		extensions:       append([]projecttypeenv.ProjectTypeEnvExtensionArtifact(nil), orderedExtensions...),
		runtime:          verifiedRuntime,
		mechanisms:       append([]runtimemechanism.RuntimeMechanismArtifactV1(nil), verifiedMechanisms...),
		policies:         append([]projecttypeenv.RegistrationPolicyArtifact(nil), verifiedPolicies...),
		composite:        verifiedComposite,
		records:          cloneArtifactRecords(records),
		mechanismRecords: cloneRuntimeMechanismRecords(mechanismRecords),
		policyRecords:    cloneRegistrationPolicyRecords(policyRecords),
	}, nil
}

type artifactClosureCompositeSealer func(
	projecttypeenv.LinkedProjectTypeEnvCompositeIR,
	projecttypeenv.RuntimeEvaluationBasisArtifact,
) (projecttypeenv.ProjectTypeEnvCompositeArtifact, error)

func resealArtifactClosureComposite(
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
	runtime projecttypeenv.RuntimeEvaluationBasisArtifact,
	composite projecttypeenv.ProjectTypeEnvCompositeArtifact,
) (projecttypeenv.ProjectTypeEnvCompositeArtifact, error) {
	sealers := map[string]artifactClosureCompositeSealer{
		projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV1: projecttypeenv.ResealHistoricalProjectTypeEnvCompositeV1,
		projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV2: projecttypeenv.SealProjectTypeEnvComposite,
	}
	sealer, present := sealers[composite.LowererSchemaVersion()]
	if !present {
		return projecttypeenv.ProjectTypeEnvCompositeArtifact{}, fmt.Errorf(
			"unsupported project TypeEnv composite lowerer schema %q",
			composite.LowererSchemaVersion(),
		)
	}
	return sealer(linked, runtime)
}

func (closure ArtifactClosure) Base() typeenv.BaseTypeEnvArtifact {
	return closure.base
}

func (closure ArtifactClosure) Extensions() []projecttypeenv.ProjectTypeEnvExtensionArtifact {
	return append([]projecttypeenv.ProjectTypeEnvExtensionArtifact(nil), closure.extensions...)
}

func (closure ArtifactClosure) RuntimeBasis() projecttypeenv.RuntimeEvaluationBasisArtifact {
	return closure.runtime
}

func (closure ArtifactClosure) RuntimeMechanisms() []runtimemechanism.RuntimeMechanismArtifactV1 {
	return append([]runtimemechanism.RuntimeMechanismArtifactV1(nil), closure.mechanisms...)
}

func (closure ArtifactClosure) RegistrationPolicies() []projecttypeenv.RegistrationPolicyArtifact {
	return append([]projecttypeenv.RegistrationPolicyArtifact(nil), closure.policies...)
}

func (closure ArtifactClosure) Composite() projecttypeenv.ProjectTypeEnvCompositeArtifact {
	return closure.composite
}

func verifyArtifactClosure(closure ArtifactClosure) (ArtifactClosure, error) {
	return PrepareArtifactClosureWithRuntimeClosure(
		closure.Base(),
		closure.Extensions(),
		closure.RuntimeBasis(),
		closure.Composite(),
		closure.RuntimeMechanisms(),
		closure.RegistrationPolicies(),
	)
}

func cloneArtifactRecords(records []artifactRecord) []artifactRecord {
	result := make([]artifactRecord, 0, len(records))
	for _, record := range records {
		result = append(result, record.clone())
	}
	return result
}

func cloneRuntimeMechanismRecords(
	records []runtimeMechanismRecord,
) []runtimeMechanismRecord {
	result := make([]runtimeMechanismRecord, 0, len(records))
	for _, record := range records {
		result = append(result, record.clone())
	}
	return result
}

func cloneRegistrationPolicyRecords(
	records []registrationPolicyRecord,
) []registrationPolicyRecord {
	result := make([]registrationPolicyRecord, 0, len(records))
	for _, record := range records {
		result = append(result, record.clone())
	}
	return result
}

func closureLinkError(issues []projecttypeenv.LinkIssue) error {
	if len(issues) == 0 {
		return fmt.Errorf("%w: B/E link rejected", ErrClosureInconsistent)
	}
	issue := issues[0]
	return fmt.Errorf(
		"%w: B/E link rejected with %s at %s: %s",
		ErrClosureInconsistent,
		issue.Code(),
		issue.Location().String(),
		issue.Detail(),
	)
}
