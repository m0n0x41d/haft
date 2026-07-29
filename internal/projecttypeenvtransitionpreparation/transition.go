// Package projecttypeenvtransitionpreparation derives immutable post-Genesis
// project-TypeEnv candidates without selecting a project head or granting
// authority.
package projecttypeenvtransitionpreparation

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionrevalidation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
)

// CandidateInput contains exact immutable observations only. It cannot express
// a head mutation, human act, or persistence effect.
type CandidateInput struct {
	Project            projectidentity.ProjectID
	ProjectRoot        projectprofile.ProjectRootV1
	PriorHead          projecttypeenvselection.ProjectTypeEnvHeadState
	PriorExecutable    projecttypeenv.ProjectTypeEnvExecutableSnapshot
	Target             localpracticeruntime.Target
	CurrentGraph       projectgraphobservation.CurrentProjectGraphObservation
	ReferenceKindFacts projectgraphobservation.ExactTargetReferenceKindFactView
	CurrentProfile     projecttypeenvprofilebasis.CurrentProjectProfileBasis
}

// Candidate is one complete non-binding successor preparation. Projection
// profile compatibility is review material and never silently rewrites an
// immutable profile edition.
type Candidate struct {
	priorHead            projecttypeenvselection.ProjectTypeEnvHeadState
	target               localpracticeruntime.Target
	closure              projecttypeenvstore.ArtifactClosure
	stage                projecttypeenvselection.ProjectTypeEnvStage
	verification         projecttypeenv.ProjectTypeEnvCompositeVerification
	snapshot             projecttypeenv.ProjectTypeEnvExecutableSnapshot
	runtime              projecttypeenvruntime.ExactTargetRuntimeRegistry
	successorDiff        projecttypeenvcompatibility.SuccessorDiff
	profileCompatibility projecttypeenvprofilecompatibility.TransitionProjectionProfileCompatibilitySet
}

func (candidate Candidate) PriorHead() projecttypeenvselection.ProjectTypeEnvHeadState {
	return candidate.priorHead
}

func (candidate Candidate) Target() localpracticeruntime.Target { return candidate.target }

func (candidate Candidate) ArtifactClosure() projecttypeenvstore.ArtifactClosure {
	return candidate.closure
}

func (candidate Candidate) Stage() projecttypeenvselection.ProjectTypeEnvStage {
	return candidate.stage
}

func (candidate Candidate) Verification() projecttypeenv.ProjectTypeEnvCompositeVerification {
	return candidate.verification
}

func (candidate Candidate) ExecutableSnapshot() projecttypeenv.ProjectTypeEnvExecutableSnapshot {
	return candidate.snapshot
}

func (candidate Candidate) ExactRuntime() projecttypeenvruntime.ExactTargetRuntimeRegistry {
	return candidate.runtime
}

func (candidate Candidate) SuccessorDiff() projecttypeenvcompatibility.SuccessorDiff {
	decoded, _ := projecttypeenvcompatibility.DecodeSuccessorDiff(
		candidate.successorDiff.CanonicalBytes(),
	)
	return decoded
}

func (candidate Candidate) ProjectionProfiles() projecttypeenvprofilecompatibility.ProjectionProfileCompatibilitySet {
	decoded, _ := projecttypeenvprofilecompatibility.DecodeTransitionProjectionProfiles(
		candidate.profileCompatibility,
	)
	return decoded
}

func (candidate Candidate) TransitionProjectionProfiles() projecttypeenvprofilecompatibility.TransitionProjectionProfileCompatibilitySet {
	decoded, _ := projecttypeenvprofilecompatibility.DecodeTransitionProjectionProfileCompatibilitySet(
		candidate.profileCompatibility.CanonicalBytes(),
	)
	return decoded
}

func (candidate Candidate) Verify() error {
	if err := candidate.priorHead.Verify(); err != nil {
		return fmt.Errorf("verify Transition prior head: %w", err)
	}
	if err := candidate.stage.Verify(); err != nil {
		return fmt.Errorf("verify project TypeEnv Transition Stage: %w", err)
	}
	if err := candidate.verification.Verify(); err != nil {
		return fmt.Errorf("verify project TypeEnv Transition final lowering: %w", err)
	}
	if err := candidate.snapshot.Verify(); err != nil {
		return fmt.Errorf("verify project TypeEnv Transition executable snapshot: %w", err)
	}
	if !candidate.runtime.Valid() {
		return fmt.Errorf("verify project TypeEnv Transition: exact runtime is absent")
	}
	if err := candidate.successorDiff.Verify(); err != nil {
		return fmt.Errorf("verify project TypeEnv successor diff: %w", err)
	}
	if err := candidate.profileCompatibility.Verify(); err != nil {
		return fmt.Errorf("verify projection-profile compatibility set: %w", err)
	}
	predecessor, ok := candidate.stage.Predecessor().(projecttypeenvselection.TransitionStagePredecessor)
	if !ok {
		return fmt.Errorf("verify project TypeEnv Transition: exact prior head is absent")
	}
	if predecessor.Head() != candidate.priorHead.Ref() ||
		predecessor.HeadRevision() != candidate.priorHead.Revision() ||
		predecessor.SelectedComposite() != candidate.priorHead.SelectedComposite() {
		return fmt.Errorf("verify project TypeEnv Transition: Stage prior head mismatch")
	}
	baseRef, executable := candidate.closure.Base().TypeEnvRef()
	targetBaseRef, targetExecutable := candidate.target.Base().TypeEnvRef()
	if !executable ||
		!targetExecutable ||
		baseRef != targetBaseRef ||
		candidate.closure.RuntimeBasis().Ref() != candidate.target.RuntimeBasis().Ref() ||
		candidate.closure.Composite().Ref() != candidate.target.Composite().Ref() {
		return fmt.Errorf("verify project TypeEnv Transition: B/E/X/C closure mismatch")
	}
	extensions := candidate.closure.Extensions()
	if len(extensions) != 1 || extensions[0].Ref() != candidate.target.Extension().Ref() {
		return fmt.Errorf("verify project TypeEnv Transition: ordered E closure mismatch")
	}
	runtimeBasis, present := candidate.runtime.RuntimeBasisRef()
	if !present || runtimeBasis != candidate.target.RuntimeBasis().Ref() {
		return fmt.Errorf("verify project TypeEnv Transition: exact runtime X mismatch")
	}
	if candidate.verification.CompositeRef() != candidate.target.Composite().Ref() ||
		candidate.snapshot.TypeEnvRef() != candidate.target.Composite().Ref() ||
		candidate.stage.CompositeVerificationRef() != candidate.verification.Ref() ||
		candidate.stage.VerifiedComposite() != candidate.snapshot.TypeEnvRef() {
		return fmt.Errorf("verify project TypeEnv Transition: C/verification/snapshot/Stage mismatch")
	}
	stageProfiles, present := candidate.stage.TransitionProjectionProfileCompatibility()
	if !present || !bytes.Equal(
		stageProfiles.CanonicalBytes(),
		candidate.profileCompatibility.CanonicalBytes(),
	) {
		return fmt.Errorf("verify project TypeEnv Transition: Stage profile-compatibility artifact mismatch")
	}
	if candidate.successorDiff.Base() != candidate.priorHead.SelectedComposite() ||
		candidate.successorDiff.Target() != candidate.snapshot.TypeEnvRef() ||
		candidate.profileCompatibility.SuccessorDiff().Digest() != candidate.successorDiff.Digest() {
		return fmt.Errorf("verify project TypeEnv Transition: successor review basis mismatch")
	}
	return nil
}

// PrepareCandidate derives one exact successor Stage from an already compiled
// target and exact current observations. It performs no IO and cannot select
// which source bytes to compile.
func PrepareCandidate(input CandidateInput) (Candidate, error) {
	project, err := projectidentity.ParseProjectID(input.Project.String())
	if err != nil || project != input.Project {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition: exact project identity is required")
	}
	root, err := projectprofile.NewProjectRootV1(input.ProjectRoot.String())
	if err != nil || root != input.ProjectRoot {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition: exact project root is required")
	}
	if err := input.PriorHead.Verify(); err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition prior head: %w", err)
	}
	if input.PriorHead.Project() != project {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition: prior-head project mismatch")
	}
	if err := input.PriorExecutable.Verify(); err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition prior executable: %w", err)
	}
	if input.PriorExecutable.TypeEnvRef() != input.PriorHead.SelectedComposite() {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition: prior executable differs from selected C")
	}
	if err := input.CurrentGraph.Verify(); err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition current graph: %w", err)
	}
	graphBasis := input.CurrentGraph.GraphSnapshotBasis()
	if graphBasis.Project() != project ||
		input.CurrentGraph.ActiveTypeEnv() != input.PriorHead.SelectedComposite() {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition: current graph does not use prior selected C")
	}
	if input.CurrentProfile == nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition: current project profile basis is required")
	}
	if err := input.CurrentProfile.Verify(); err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition current profile: %w", err)
	}
	if input.CurrentProfile.ProjectRoot() != root {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition: current profile root mismatch")
	}
	verification, snapshot, err := requirePreparedTarget(input.Target)
	if err != nil {
		return Candidate{}, err
	}
	if snapshot.TypeEnvRef() == input.PriorHead.SelectedComposite() {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition: bundled target is already selected")
	}
	runtime, err := requireExactTargetRuntime(input.Target)
	if err != nil {
		return Candidate{}, err
	}
	closure, err := projecttypeenvstore.PrepareArtifactClosureWithRuntimeClosure(
		input.Target.Base(),
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{input.Target.Extension()},
		input.Target.RuntimeBasis(),
		input.Target.Composite(),
		[]runtimemechanism.RuntimeMechanismArtifactV1{input.Target.Mechanism()},
		input.Target.RegistrationPolicies(),
	)
	if err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition artifact closure: %w", err)
	}
	stageDiff, err := projecttypeenvcompatibility.Compare(
		input.PriorExecutable.Environment(),
		snapshot.Environment(),
	)
	if err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition compatibility: %w", err)
	}
	stageCompatibility, err := projecttypeenvselection.NewComparedStageCompatibility(stageDiff)
	if err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition Stage compatibility: %w", err)
	}
	successorDiff, err := projecttypeenvcompatibility.CompareSuccessor(
		input.PriorExecutable.Environment(),
		snapshot.Environment(),
	)
	if err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv successor diff: %w", err)
	}
	profiles, err := projecttypeenvprofilecompatibility.AssessTransitionProjectionProfiles(
		successorDiff,
	)
	if err != nil {
		return Candidate{}, fmt.Errorf("prepare installed projection-profile compatibility: %w", err)
	}
	revalidation, err := projecttypeenvassertionrevalidation.Revalidate(
		projecttypeenvassertionrevalidation.Input{
			CurrentGraph:                  input.CurrentGraph,
			TargetTypeEnv:                 snapshot.Environment(),
			TargetRuntime:                 runtime,
			ExactTargetReferenceKindFacts: input.ReferenceKindFacts,
		},
	)
	if err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition assertion revalidation: %w", err)
	}
	profileFit, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFit(
		input.CurrentProfile,
		snapshot,
	)
	if err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition profile fit: %w", err)
	}
	predecessor, err := input.PriorHead.ExactPriorHead()
	if err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition predecessor: %w", err)
	}
	stage, err := projecttypeenvselection.SealProjectTypeEnvStage(
		projecttypeenvselection.ProjectTypeEnvStageInput{
			Project:                                  project,
			Predecessor:                              predecessor,
			Base:                                     verification.BaseTypeEnvRef(),
			OrderedExtensions:                        verification.ExtensionRefs(),
			RuntimeBasis:                             verification.RuntimeEvaluationBasisRef(),
			VerifiedComposite:                        verification,
			Composite:                                verification.CompositeRef(),
			GraphSnapshotBasis:                       graphBasis,
			GraphSnapshotBasisRef:                    graphBasis.Ref(),
			GraphSnapshotBasisDigest:                 graphBasis.Ref().Digest(),
			GraphRevision:                            graphBasis.GraphRevision(),
			ProfileLedgerRevision:                    input.CurrentProfile.LedgerRevision(),
			ProfileLedgerDigest:                      input.CurrentProfile.ProfileLedgerDigest(),
			Compatibility:                            stageCompatibility,
			ExistingAssertionRevalidation:            revalidation,
			ProfileCompatibility:                     profileFit,
			TransitionProjectionProfileCompatibility: profiles,
		},
	)
	if err != nil {
		return Candidate{}, fmt.Errorf("prepare project TypeEnv Transition Stage: %w", err)
	}
	candidate := Candidate{
		priorHead:            input.PriorHead,
		target:               input.Target,
		closure:              closure,
		stage:                stage,
		verification:         verification,
		snapshot:             snapshot,
		runtime:              runtime,
		successorDiff:        successorDiff,
		profileCompatibility: profiles,
	}
	if err := candidate.Verify(); err != nil {
		return Candidate{}, err
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
			fmt.Errorf("prepare project TypeEnv Transition: target has no executable preparation")
	}
	verification, verified := preparation.Verification()
	snapshot, executable := preparation.ExecutableSnapshot()
	if !verified || !executable {
		return projecttypeenv.ProjectTypeEnvCompositeVerification{},
			projecttypeenv.ProjectTypeEnvExecutableSnapshot{},
			fmt.Errorf("prepare project TypeEnv Transition: target verification or executable snapshot is absent")
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
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{},
			fmt.Errorf("prepare project TypeEnv Transition: installed target runtime is %s", observation.Kind())
	}
	runtime, present := matched.Registry()
	if !present || !runtime.Valid() {
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{},
			fmt.Errorf("prepare project TypeEnv Transition: exact target runtime is absent")
	}
	return runtime, nil
}
