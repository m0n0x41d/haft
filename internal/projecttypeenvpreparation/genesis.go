// Package projecttypeenvpreparation derives immutable project-TypeEnv
// candidates without selecting a project head or granting authority.
package projecttypeenvpreparation

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionrevalidation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

// GenesisCandidateInput contains only exact immutable observations. It cannot
// express a project-head selection, a human act, or a persistence effect.
type GenesisCandidateInput struct {
	Project        projectidentity.ProjectID
	ProjectRoot    projectprofile.ProjectRootV1
	Target         localpracticeruntime.Target
	CurrentGraph   projectgraphobservation.CurrentProjectGraphObservation
	CurrentProfile projecttypeenvprofilebasis.CurrentProjectProfileBasis
}

// GenesisCandidate is the complete non-binding B/E/X/C/Stage preparation for
// one observed project graph and profile. Possessing it grants no write or
// project-head-selection capability.
type GenesisCandidate struct {
	baseSnapshot typedmemorystore.TypeEnvSnapshot
	target       localpracticeruntime.Target
	closure      projecttypeenvstore.ArtifactClosure
	stage        projecttypeenvselection.ProjectTypeEnvStage
	verification projecttypeenv.ProjectTypeEnvCompositeVerification
	snapshot     projecttypeenv.ProjectTypeEnvExecutableSnapshot
	runtime      projecttypeenvruntime.ExactTargetRuntimeRegistry
}

func (candidate GenesisCandidate) BaseSnapshot() typedmemorystore.TypeEnvSnapshot {
	return candidate.baseSnapshot
}

func (candidate GenesisCandidate) Target() localpracticeruntime.Target {
	return candidate.target
}

func (candidate GenesisCandidate) ArtifactClosure() projecttypeenvstore.ArtifactClosure {
	return candidate.closure
}

func (candidate GenesisCandidate) Stage() projecttypeenvselection.ProjectTypeEnvStage {
	return candidate.stage
}

func (candidate GenesisCandidate) Verification() projecttypeenv.ProjectTypeEnvCompositeVerification {
	return candidate.verification
}

func (candidate GenesisCandidate) ExecutableSnapshot() projecttypeenv.ProjectTypeEnvExecutableSnapshot {
	return candidate.snapshot
}

func (candidate GenesisCandidate) ExactRuntime() projecttypeenvruntime.ExactTargetRuntimeRegistry {
	return candidate.runtime
}

func (candidate GenesisCandidate) Verify() error {
	if err := candidate.stage.Verify(); err != nil {
		return fmt.Errorf("verify project TypeEnv Genesis Stage: %w", err)
	}
	if err := candidate.verification.Verify(); err != nil {
		return fmt.Errorf("verify project TypeEnv Genesis final lowering: %w", err)
	}
	if err := candidate.snapshot.Verify(); err != nil {
		return fmt.Errorf("verify project TypeEnv Genesis executable snapshot: %w", err)
	}
	if !candidate.runtime.Valid() {
		return fmt.Errorf("verify project TypeEnv Genesis: exact runtime is absent")
	}
	baseRef, executable := candidate.target.Base().TypeEnvRef()
	if !executable ||
		candidate.baseSnapshot.Ref() != baseRef ||
		candidate.closure.Base().Digest() != candidate.target.Base().Digest() ||
		candidate.closure.RuntimeBasis().Ref() != candidate.target.RuntimeBasis().Ref() ||
		candidate.closure.Composite().Ref() != candidate.target.Composite().Ref() {
		return fmt.Errorf("verify project TypeEnv Genesis: B/E/X/C closure mismatch")
	}
	extensions := candidate.closure.Extensions()
	if len(extensions) != 1 ||
		extensions[0].Ref() != candidate.target.Extension().Ref() {
		return fmt.Errorf("verify project TypeEnv Genesis: ordered E closure mismatch")
	}
	runtimeBasis, present := candidate.runtime.RuntimeBasisRef()
	if !present || runtimeBasis != candidate.target.RuntimeBasis().Ref() {
		return fmt.Errorf("verify project TypeEnv Genesis: exact runtime X mismatch")
	}
	if candidate.verification.CompositeRef() != candidate.target.Composite().Ref() ||
		candidate.snapshot.TypeEnvRef() != candidate.target.Composite().Ref() ||
		candidate.stage.CompositeVerificationRef() != candidate.verification.Ref() ||
		candidate.stage.VerifiedComposite() != candidate.snapshot.TypeEnvRef() {
		return fmt.Errorf("verify project TypeEnv Genesis: C/verification/snapshot/Stage mismatch")
	}
	return nil
}

// PrepareGenesisCandidate derives the exact initial Stage from an already
// compiled B/E/X/C target and supplied current observations. It performs no IO
// and cannot choose which source carrier or target recipe to compile.
func PrepareGenesisCandidate(
	input GenesisCandidateInput,
) (GenesisCandidate, error) {
	project, err := projectidentity.ParseProjectID(input.Project.String())
	if err != nil || project != input.Project {
		return GenesisCandidate{}, fmt.Errorf(
			"prepare project TypeEnv Genesis: exact project identity is required",
		)
	}
	if err := input.CurrentGraph.Verify(); err != nil {
		return GenesisCandidate{}, fmt.Errorf(
			"prepare project TypeEnv Genesis current graph: %w",
			err,
		)
	}
	if input.CurrentGraph.GraphSnapshotBasis().Project() != project {
		return GenesisCandidate{}, fmt.Errorf(
			"prepare project TypeEnv Genesis: current graph project mismatch",
		)
	}
	if input.CurrentGraph.GraphSnapshotBasis().GraphRevision().Value() != 0 {
		return GenesisCandidate{}, fmt.Errorf(
			"prepare project TypeEnv Genesis: current graph must be at revision zero",
		)
	}
	if input.CurrentProfile == nil {
		return GenesisCandidate{}, fmt.Errorf(
			"prepare project TypeEnv Genesis: current project profile basis is required",
		)
	}
	if err := input.CurrentProfile.Verify(); err != nil {
		return GenesisCandidate{}, fmt.Errorf(
			"prepare project TypeEnv Genesis current profile: %w",
			err,
		)
	}
	root, err := projectprofile.NewProjectRootV1(input.ProjectRoot.String())
	if err != nil || root != input.ProjectRoot {
		return GenesisCandidate{}, fmt.Errorf(
			"prepare project TypeEnv Genesis: exact project root is required",
		)
	}
	if input.CurrentProfile.ProjectRoot() != root {
		return GenesisCandidate{}, fmt.Errorf(
			"prepare project TypeEnv Genesis: current profile project root mismatch",
		)
	}
	baseSnapshot, err := projectmemory.NewBaseTypeEnvSnapshot(input.Target.Base())
	if err != nil {
		return GenesisCandidate{}, err
	}
	if input.CurrentGraph.ActiveTypeEnv() != baseSnapshot.Ref() {
		return GenesisCandidate{}, fmt.Errorf(
			"prepare project TypeEnv Genesis: current graph active TypeEnv %q is not exact base %q",
			input.CurrentGraph.ActiveTypeEnv().String(),
			baseSnapshot.Ref().String(),
		)
	}
	target := input.Target
	verification, snapshot, err := requirePreparedTarget(target)
	if err != nil {
		return GenesisCandidate{}, err
	}
	runtime, err := requireExactTargetRuntime(target)
	if err != nil {
		return GenesisCandidate{}, err
	}
	closure, err := projecttypeenvstore.PrepareArtifactClosureWithRuntimeClosure(
		target.Base(),
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{target.Extension()},
		target.RuntimeBasis(),
		target.Composite(),
		[]runtimemechanism.RuntimeMechanismArtifactV1{target.Mechanism()},
		target.RegistrationPolicies(),
	)
	if err != nil {
		return GenesisCandidate{}, fmt.Errorf(
			"prepare project TypeEnv Genesis artifact closure: %w",
			err,
		)
	}
	stage, err := sealGenesisStage(
		input,
		target,
		verification,
		snapshot,
		runtime,
	)
	if err != nil {
		return GenesisCandidate{}, err
	}
	candidate := GenesisCandidate{
		baseSnapshot: baseSnapshot,
		target:       target,
		closure:      closure,
		stage:        stage,
		verification: verification,
		snapshot:     snapshot,
		runtime:      runtime,
	}
	if err := candidate.Verify(); err != nil {
		return GenesisCandidate{}, err
	}
	return candidate, nil
}

func requirePreparedTarget(
	target localpracticeruntime.Target,
) (
	projecttypeenv.ProjectTypeEnvCompositeVerification,
	projecttypeenv.ProjectTypeEnvExecutableSnapshot,
	error,
) {
	preparation := target.Preparation()
	if preparation == nil || preparation.Rejected() {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{},
			projecttypeenv.ProjectTypeEnvExecutableSnapshot{},
			fmt.Errorf(
				"prepare project TypeEnv Genesis: target has no executable preparation",
			)
	}
	verification, verified := preparation.Verification()
	snapshot, executable := preparation.ExecutableSnapshot()
	if !verified || !executable {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{},
			projecttypeenv.ProjectTypeEnvExecutableSnapshot{},
			fmt.Errorf(
				"prepare project TypeEnv Genesis: target verification or executable snapshot is absent",
			)
	}
	return verification, snapshot, nil
}

func requireExactTargetRuntime(
	target localpracticeruntime.Target,
) (projecttypeenvruntime.ExactTargetRuntimeRegistry, error) {
	observation := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: target.RuntimeBasis(),
			Installed:    target.InstalledRuntime(),
		},
	)
	matched, ok := observation.(projecttypeenvruntime.Matched)
	if !ok {
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{}, fmt.Errorf(
			"prepare project TypeEnv Genesis: installed target runtime is %s",
			observation.Kind(),
		)
	}
	runtime, present := matched.Registry()
	if !present || !runtime.Valid() {
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{}, fmt.Errorf(
			"prepare project TypeEnv Genesis: exact target runtime is absent",
		)
	}
	return runtime, nil
}

func sealGenesisStage(
	input GenesisCandidateInput,
	target localpracticeruntime.Target,
	verification projecttypeenv.ProjectTypeEnvCompositeVerification,
	snapshot projecttypeenv.ProjectTypeEnvExecutableSnapshot,
	runtime projecttypeenvruntime.ExactTargetRuntimeRegistry,
) (projecttypeenvselection.ProjectTypeEnvStage, error) {
	revalidation, err := projecttypeenvassertionrevalidation.Revalidate(
		projecttypeenvassertionrevalidation.Input{
			CurrentGraph:  input.CurrentGraph,
			TargetTypeEnv: snapshot.Environment(),
			TargetRuntime: runtime,
		},
	)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvStage{}, fmt.Errorf(
			"prepare project TypeEnv Genesis assertion revalidation: %w",
			err,
		)
	}
	profileFit, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFit(
		input.CurrentProfile,
		snapshot,
	)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvStage{}, fmt.Errorf(
			"prepare project TypeEnv Genesis profile fit: %w",
			err,
		)
	}
	compatibility, err := projecttypeenvselection.NewInitialStageCompatibility(
		verification.CompositeRef(),
	)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvStage{}, fmt.Errorf(
			"prepare project TypeEnv Genesis compatibility: %w",
			err,
		)
	}
	graphBasis := input.CurrentGraph.GraphSnapshotBasis()
	stage, err := projecttypeenvselection.SealProjectTypeEnvStage(
		projecttypeenvselection.ProjectTypeEnvStageInput{
			Project:                       input.Project,
			Predecessor:                   projecttypeenvselection.NewGenesisStagePredecessor(),
			Base:                          verification.BaseTypeEnvRef(),
			OrderedExtensions:             verification.ExtensionRefs(),
			RuntimeBasis:                  verification.RuntimeEvaluationBasisRef(),
			VerifiedComposite:             verification,
			Composite:                     verification.CompositeRef(),
			GraphSnapshotBasis:            graphBasis,
			GraphSnapshotBasisRef:         graphBasis.Ref(),
			GraphSnapshotBasisDigest:      graphBasis.Ref().Digest(),
			GraphRevision:                 graphBasis.GraphRevision(),
			ProfileLedgerRevision:         input.CurrentProfile.LedgerRevision(),
			ProfileLedgerDigest:           input.CurrentProfile.ProfileLedgerDigest(),
			Compatibility:                 compatibility,
			ExistingAssertionRevalidation: revalidation,
			ProfileCompatibility:          profileFit,
		},
	)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvStage{}, fmt.Errorf(
			"prepare project TypeEnv Genesis Stage: %w",
			err,
		)
	}
	return stage, nil
}
