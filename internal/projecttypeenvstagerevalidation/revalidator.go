package projecttypeenvstagerevalidation

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionrevalidation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ProjectTypeEnvStageRevalidationInput accepts the current graph only through
// the exact projectgraphobservation value produced by the transaction adapter.
// TargetRuntimeRegistry is an in-process capability produced by exact
// projecttypeenvruntime observation; a zero value means that derivation remains
// unavailable. CurrentProfile is the exact current profile basis read in the
// same transaction. PriorExecutable is required only for Transition and must
// be the exact immutable C selected by the observed predecessor head.
// Authority and head effects stay outside this pure comparison.
type ProjectTypeEnvStageRevalidationInput struct {
	Stage                 projecttypeenvselection.ProjectTypeEnvStage
	FinalVerification     projecttypeenv.ProjectTypeEnvCompositeVerification
	ExecutableTarget      projecttypeenv.ProjectTypeEnvExecutableSnapshot
	TargetRuntimeRegistry projecttypeenvruntime.ExactTargetRuntimeRegistry
	CurrentGraph          projectgraphobservation.CurrentProjectGraphObservation
	ReferenceKindFacts    projectgraphobservation.ExactTargetReferenceKindFactView
	CurrentProfile        projecttypeenvprofilebasis.CurrentProjectProfileBasis
	CurrentHead           CurrentProjectTypeEnvHeadObservation
	PriorExecutable       projecttypeenv.ProjectTypeEnvExecutableSnapshot
}

// Revalidate recomputes every mutable Stage basis from exact current inputs.
// For Transition it also recomputes the complete executable-TypeEnv diff from
// the prior selected immutable C. A clean assertion report and compatible
// project-profile assessment mint an opaque CurrentSelectionStage.
func Revalidate(
	input ProjectTypeEnvStageRevalidationInput,
) StageRevalidationResult {
	inputIssues := collectInvalidInputIssues(input)
	if len(inputIssues) > 0 {
		return newInvalidResult(inputIssues)
	}
	structuralIssues := make([]StageRevalidationIssue, 0)
	structuralIssues = append(structuralIssues, compareTargetClosure(input)...)
	structuralIssues = append(structuralIssues, compareCurrentGraph(input)...)
	structuralIssues = append(structuralIssues, comparePredecessor(input)...)
	if len(structuralIssues) > 0 {
		return newDriftedResult(structuralIssues)
	}
	trustedComparison := CompareCurrentTrustedStageEditions(
		input.Stage,
		input.ExecutableTarget,
	)
	switch trusted := trustedComparison.(type) {
	case InvalidTrustedStageEditionInput:
		return newInvalidResult(
			trustedStageEditionInputIssuesForRevalidation(trusted),
		)
	case UnsupportedTrustedStageEditions:
		return newDriftedResult(
			unsupportedTrustedStageEditionIssues(trusted),
		)
	case StaticTrustedStageEditionsMatched:
		runtimeIssues := compareTargetRuntimeRegistry(
			trusted.RuntimeRegistryRequirement(),
			input.TargetRuntimeRegistry,
		)
		if len(runtimeIssues) > 0 {
			return newDriftedResult(runtimeIssues)
		}
		if !input.TargetRuntimeRegistry.Valid() {
			return newUnavailableResult(
				[]DerivedInputRequirement{
					RequirementTargetRuntimeRegistry,
				},
			)
		}
		compatibilityResult := revalidateTransitionCompatibility(input)
		switch compatibility := compatibilityResult.(type) {
		case transitionCompatibilityMatched:
		case transitionCompatibilityUnavailable:
			return newUnavailableResult(
				[]DerivedInputRequirement{RequirementTypeEnvCompatibility},
			)
		case transitionCompatibilityInvalid:
			return newInvalidResult(compatibility.issues)
		case transitionCompatibilityDrifted:
			return newDriftedResult(compatibility.issues)
		case transitionCompatibilityRejected:
			return newRejectedResult(compatibility.issues)
		default:
			issue := newIssue(
				IssueTypeEnvCompatibilityInvalid,
				"transition compatibility revalidation",
				"one closed compatibility result",
				fmt.Sprintf("%T", compatibilityResult),
				"repair the package-owned transition compatibility producer",
			)
			return newInvalidResult([]StageRevalidationIssue{issue})
		}
		derived, derivedIssues := deriveCurrentSelectionInputs(input)
		if len(derivedIssues) > 0 {
			return newInvalidResult(derivedIssues)
		}
		currentBasisIssues := compareCurrentSelectionBasis(input, derived)
		if len(currentBasisIssues) > 0 {
			return newDriftedResult(currentBasisIssues)
		}
		readinessIssues := currentSelectionReadinessIssues(input, derived)
		if len(readinessIssues) > 0 {
			return newRejectedResult(readinessIssues)
		}
		current, err := newCurrentSelectionStage(
			input.Stage,
			derived.assertions,
			input.CurrentProfile,
			derived.profile,
		)
		if err != nil {
			issue := newIssue(
				IssueCurrentSelectionStageInvalid,
				"current selection Stage capability",
				"an exact clean Genesis Stage capability",
				err.Error(),
				"rerun current Stage revalidation from exact transaction-local inputs",
			)
			return newInvalidResult([]StageRevalidationIssue{issue})
		}
		return current
	default:
		issue := newIssue(
			IssueTrustedEditionInputInvalid,
			"trusted Stage edition comparison",
			"one closed trusted-edition comparison variant",
			fmt.Sprintf("%T", trustedComparison),
			"repair the package-owned trusted-edition comparison",
		)
		return newInvalidResult([]StageRevalidationIssue{issue})
	}
}

type transitionCompatibilityResult interface {
	transitionCompatibilityResultVariant()
}

type transitionCompatibilityMatched struct{}

func (transitionCompatibilityMatched) transitionCompatibilityResultVariant() {}

type transitionCompatibilityUnavailable struct{}

func (transitionCompatibilityUnavailable) transitionCompatibilityResultVariant() {}

type transitionCompatibilityInvalid struct {
	issues []StageRevalidationIssue
}

func (transitionCompatibilityInvalid) transitionCompatibilityResultVariant() {}

type transitionCompatibilityDrifted struct {
	issues []StageRevalidationIssue
}

func (transitionCompatibilityDrifted) transitionCompatibilityResultVariant() {}

type transitionCompatibilityRejected struct {
	issues []StageRevalidationIssue
}

func (transitionCompatibilityRejected) transitionCompatibilityResultVariant() {}

func revalidateTransitionCompatibility(
	input ProjectTypeEnvStageRevalidationInput,
) transitionCompatibilityResult {
	predecessor, transition := input.Stage.Predecessor().(projecttypeenvselection.TransitionStagePredecessor)
	if !transition {
		return transitionCompatibilityMatched{}
	}
	priorRecord := input.PriorExecutable.Record()
	if len(priorRecord.CanonicalBytes()) == 0 {
		return transitionCompatibilityUnavailable{}
	}
	if err := input.PriorExecutable.Verify(); err != nil {
		issue := newIssue(
			IssuePriorExecutableSnapshotInvalid,
			"prior selected executable C",
			"an exact restored executable snapshot capability",
			err.Error(),
			"reload the prior selected C from its immutable B/E/X/C closure",
		)
		return transitionCompatibilityInvalid{
			issues: []StageRevalidationIssue{issue},
		}
	}
	if input.PriorExecutable.TypeEnvRef() != predecessor.SelectedComposite() {
		issue := newIssue(
			IssuePriorExecutableSnapshotMismatch,
			"prior executable C",
			predecessor.SelectedComposite().String(),
			input.PriorExecutable.TypeEnvRef().String(),
			"reload the exact C selected by the Transition predecessor",
		)
		return transitionCompatibilityDrifted{
			issues: []StageRevalidationIssue{issue},
		}
	}
	expected, compared := input.Stage.Compatibility().(projecttypeenvselection.ComparedStageCompatibility)
	if !compared {
		issue := newIssue(
			IssueTypeEnvCompatibilityInvalid,
			"Transition Stage compatibility",
			"one complete compared executable-TypeEnv diff",
			fmt.Sprintf("%T", input.Stage.Compatibility()),
			"rebuild the Transition Stage from the exact prior and target C",
		)
		return transitionCompatibilityInvalid{
			issues: []StageRevalidationIssue{issue},
		}
	}
	actual, err := projecttypeenvcompatibility.Compare(
		input.PriorExecutable.Environment(),
		input.ExecutableTarget.Environment(),
	)
	if err != nil {
		issue := newIssue(
			IssueTypeEnvCompatibilityInvalid,
			"current executable-TypeEnv compatibility",
			"one complete canonical diff",
			err.Error(),
			"repair the exact prior and target executable snapshots",
		)
		return transitionCompatibilityInvalid{
			issues: []StageRevalidationIssue{issue},
		}
	}
	expectedDiff := expected.Diff()
	if expectedDiff.Digest() == actual.Digest() && bytes.Equal(
		expectedDiff.CanonicalBytes(),
		actual.CanonicalBytes(),
	) {
		return revalidateTransitionProjectionProfiles(input)
	}
	issue := newIssue(
		IssueTypeEnvCompatibilityMismatch,
		"Transition executable-TypeEnv compatibility",
		expectedDiff.Digest().String(),
		actual.Digest().String(),
		"rebuild the Stage against the exact prior selected C and target C",
	)
	return transitionCompatibilityDrifted{
		issues: []StageRevalidationIssue{issue},
	}
}

func revalidateTransitionProjectionProfiles(
	input ProjectTypeEnvStageRevalidationInput,
) transitionCompatibilityResult {
	stored, exists := input.Stage.TransitionProjectionProfileCompatibility()
	if !exists {
		issue := newIssue(
			IssueProjectionProfileCompatibilityInvalid,
			"Transition projection-profile compatibility",
			"one Stage-bound complete successor/profile artifact",
			"absent",
			"rebuild the Transition Stage under Stage schema v5",
		)
		return transitionCompatibilityInvalid{issues: []StageRevalidationIssue{issue}}
	}
	diff, err := projecttypeenvcompatibility.CompareSuccessor(
		input.PriorExecutable.Environment(),
		input.ExecutableTarget.Environment(),
	)
	if err != nil {
		issue := newIssue(
			IssueProjectionProfileCompatibilityInvalid,
			"current complete TypeEnv successor diff",
			"one exact complete semantic diff",
			err.Error(),
			"repair the exact prior and target executable snapshots",
		)
		return transitionCompatibilityInvalid{issues: []StageRevalidationIssue{issue}}
	}
	actual, err := projecttypeenvprofilecompatibility.AssessTransitionProjectionProfiles(diff)
	if err != nil {
		issue := newIssue(
			IssueProjectionProfileCompatibilityInvalid,
			"current installed projection-profile compatibility",
			"one exact assessment for every installed immutable profile edition",
			err.Error(),
			"repair the installed projection-profile compatibility producer",
		)
		return transitionCompatibilityInvalid{issues: []StageRevalidationIssue{issue}}
	}
	if stored.Ref() != actual.Ref() ||
		stored.Digest() != actual.Digest() ||
		!bytes.Equal(stored.CanonicalBytes(), actual.CanonicalBytes()) {
		issue := newIssue(
			IssueProjectionProfileCompatibilityMismatch,
			"Transition installed projection-profile compatibility",
			stored.Digest().String(),
			actual.Digest().String(),
			"rebuild the Stage against the exact current installed profile editions",
		)
		return transitionCompatibilityDrifted{issues: []StageRevalidationIssue{issue}}
	}
	blocked, err := projecttypeenvprofilecompatibility.TransitionProjectionProfilesHaveBlockedProfile(
		actual,
	)
	if err != nil {
		issue := newIssue(
			IssueProjectionProfileCompatibilityInvalid,
			"current installed projection-profile compatibility",
			"one exact decodable profile compatibility set",
			err.Error(),
			"repair the installed projection-profile compatibility carrier",
		)
		return transitionCompatibilityInvalid{issues: []StageRevalidationIssue{issue}}
	}
	if blocked {
		issue := newIssue(
			IssueProjectionProfileBlocked,
			"installed immutable projection profiles",
			"no blocked profile edition",
			"one or more blocked profile editions",
			"stage a successor TypeEnv that preserves every installed profile read contract or install an explicitly reviewed successor profile edition first",
		)
		return transitionCompatibilityRejected{issues: []StageRevalidationIssue{issue}}
	}
	return transitionCompatibilityMatched{}
}

type currentSelectionInputs struct {
	assertions projecttypeenvassertionreport.Report
	profile    projecttypeenvprofilefit.Assessment
}

func deriveCurrentSelectionInputs(
	input ProjectTypeEnvStageRevalidationInput,
) (currentSelectionInputs, []StageRevalidationIssue) {
	assertions, err := projecttypeenvassertionrevalidation.Revalidate(
		projecttypeenvassertionrevalidation.Input{
			CurrentGraph:                  input.CurrentGraph,
			TargetTypeEnv:                 input.ExecutableTarget.Environment(),
			TargetRuntime:                 input.TargetRuntimeRegistry,
			ExactTargetReferenceKindFacts: input.ReferenceKindFacts,
		},
	)
	if err != nil {
		issue := newIssue(
			IssueAssertionRevalidationFailed,
			"current existing-assertion revalidation",
			"one exact report derived from current graph, C, and X",
			err.Error(),
			"repair the transaction-local assertion revalidation inputs",
		)
		return currentSelectionInputs{}, []StageRevalidationIssue{issue}
	}
	profile, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFit(
		input.CurrentProfile,
		input.ExecutableTarget,
	)
	if err != nil {
		issue := newIssue(
			IssueProfileFitAssessmentFailed,
			"current project-profile fit",
			"one exact assessment derived from current profile and C",
			err.Error(),
			"repair the transaction-local project-profile basis",
		)
		return currentSelectionInputs{}, []StageRevalidationIssue{issue}
	}
	return currentSelectionInputs{
		assertions: assertions,
		profile:    profile,
	}, nil
}

func compareCurrentSelectionBasis(
	input ProjectTypeEnvStageRevalidationInput,
	current currentSelectionInputs,
) []StageRevalidationIssue {
	stage := input.Stage
	issues := make([]StageRevalidationIssue, 0, 4)
	if stage.ProfileLedgerRevision() != input.CurrentProfile.LedgerRevision() {
		issues = append(issues, newIssue(
			IssueProfileLedgerRevisionMismatch,
			"project-profile ledger revision",
			fmt.Sprintf("%d", stage.ProfileLedgerRevision().Value()),
			fmt.Sprintf("%d", input.CurrentProfile.LedgerRevision().Value()),
			"rebuild the Stage against the exact current project-profile ledger",
		))
	}
	if stage.ProfileLedgerDigest() != input.CurrentProfile.ProfileLedgerDigest() {
		issues = append(issues, newIssue(
			IssueProfileLedgerDigestMismatch,
			"project-profile ledger digest",
			stage.ProfileLedgerDigest().String(),
			input.CurrentProfile.ProfileLedgerDigest().String(),
			"rebuild the Stage against the exact current project-profile ledger",
		))
	}
	issues = append(
		issues,
		compareAssertionRevalidationIdentity(
			stage.ExistingAssertionRevalidation(),
			current.assertions,
		)...,
	)
	issues = append(
		issues,
		compareProfileFitIdentity(
			stage.ProfileCompatibility(),
			current.profile,
		)...,
	)
	return normalizeIssues(issues)
}

func compareAssertionRevalidationIdentity(
	expected projecttypeenvassertionreport.Report,
	actual projecttypeenvassertionreport.Report,
) []StageRevalidationIssue {
	if expected.Digest() != actual.Digest() ||
		!bytes.Equal(expected.CanonicalBytes(), actual.CanonicalBytes()) {
		return []StageRevalidationIssue{newIssue(
			IssueAssertionRevalidationMismatch,
			"existing-assertion revalidation report",
			expected.Digest().String(),
			actual.Digest().String(),
			"rebuild the Stage against the current graph and exact C/X runtime",
		)}
	}
	return nil
}

func compareProfileFitIdentity(
	expected projecttypeenvprofilefit.Assessment,
	actual projecttypeenvprofilefit.Assessment,
) []StageRevalidationIssue {
	if expected.Digest() != actual.Digest() ||
		!bytes.Equal(expected.CanonicalBytes(), actual.CanonicalBytes()) {
		return []StageRevalidationIssue{newIssue(
			IssueProfileFitMismatch,
			"project-profile fit assessment",
			expected.Digest().String(),
			actual.Digest().String(),
			"rebuild the Stage against the exact current project-profile basis",
		)}
	}
	return nil
}

func currentSelectionReadinessIssues(
	input ProjectTypeEnvStageRevalidationInput,
	current currentSelectionInputs,
) []StageRevalidationIssue {
	issues := make([]StageRevalidationIssue, 0, 2)
	issues = append(
		issues,
		assertionReadinessIssues(current.assertions)...,
	)
	issues = append(
		issues,
		profileReadinessIssues(
			input.Stage,
			input.CurrentProfile,
			current.profile,
		)...,
	)
	return normalizeIssues(issues)
}

func assertionReadinessIssues(
	actual projecttypeenvassertionreport.Report,
) []StageRevalidationIssue {
	switch actual.Posture() {
	case typedmemory.RevalidationClean:
		return nil
	case typedmemory.RevalidationConflict:
		return []StageRevalidationIssue{newIssue(
			IssueAssertionRevalidationConflict,
			"current existing assertions",
			typedmemory.RevalidationClean.String(),
			actual.Posture().String(),
			"resolve or supersede the conflicting assertions before selecting C",
		)}
	case typedmemory.RevalidationUnderdetermined:
		return []StageRevalidationIssue{newIssue(
			IssueAssertionRevalidationUnderdetermined,
			"current existing assertions",
			typedmemory.RevalidationClean.String(),
			actual.Posture().String(),
			"provide the missing exact grounds before selecting C",
		)}
	default:
		return []StageRevalidationIssue{newIssue(
			IssueAssertionRevalidationFailed,
			"current existing assertions",
			"one closed revalidation posture",
			actual.Posture().String(),
			"repair the assertion revalidation report producer",
		)}
	}
}

func profileReadinessIssues(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	basis projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	actual projecttypeenvprofilefit.Assessment,
) []StageRevalidationIssue {
	if selectionProfileReady(stage, basis, actual) {
		return nil
	}
	switch actual.(type) {
	case projecttypeenvprofilefit.Compatible:
		return []StageRevalidationIssue{newIssue(
			IssueProfileFitAssessmentFailed,
			"current project profile",
			"selection-ready profile assessment",
			"compatible assessment rejected by readiness policy",
			"repair the profile readiness policy",
		)}
	case projecttypeenvprofilefit.Incompatible:
		return []StageRevalidationIssue{newIssue(
			IssueProfileIncompatible,
			"current project profile",
			"compatible",
			"incompatible",
			"select or stage a C that satisfies the declared project profile",
		)}
	case projecttypeenvprofilefit.Underdetermined:
		return []StageRevalidationIssue{newIssue(
			IssueProfileUnderdetermined,
			"current project profile",
			"compatible",
			"underdetermined",
			"complete the canonical project profile or its applicability grounds",
		)}
	case projecttypeenvprofilefit.Unavailable:
		return []StageRevalidationIssue{newIssue(
			IssueProfileUnavailable,
			"current project profile",
			"compatible",
			"unavailable",
			"install the exact profile-fit rule edition before selecting C",
		)}
	default:
		return []StageRevalidationIssue{newIssue(
			IssueProfileFitAssessmentFailed,
			"current project profile",
			"one closed profile-fit assessment variant",
			fmt.Sprintf("%T", actual),
			"repair the profile-fit assessment producer",
		)}
	}
}

func selectionProfileReady(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	_ projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	actual projecttypeenvprofilefit.Assessment,
) bool {
	if _, compatible := actual.(projecttypeenvprofilefit.Compatible); compatible {
		return true
	}
	_, underdetermined := actual.(projecttypeenvprofilefit.Underdetermined)
	return underdetermined && genesisDefaultSelection(stage)
}

func genesisDefaultSelection(
	stage projecttypeenvselection.ProjectTypeEnvStage,
) bool {
	_, genesis := stage.Predecessor().(projecttypeenvselection.GenesisStagePredecessor)
	return genesis
}

func trustedStageEditionInputIssuesForRevalidation(
	result InvalidTrustedStageEditionInput,
) []StageRevalidationIssue {
	sourceIssues := result.Issues()
	issues := make([]StageRevalidationIssue, 0, len(sourceIssues))
	for _, sourceIssue := range sourceIssues {
		issues = append(issues, newIssue(
			IssueTrustedEditionInputInvalid,
			"trusted Stage edition input "+sourceIssue.Code().String(),
			"one exact verified Stage/executable-C edition observation",
			sourceIssue.Actual(),
			sourceIssue.Repair(),
		))
	}
	return normalizeIssues(issues)
}

func unsupportedTrustedStageEditionIssues(
	result UnsupportedTrustedStageEditions,
) []StageRevalidationIssue {
	sourceIssues := result.Issues()
	issues := make([]StageRevalidationIssue, 0, len(sourceIssues))
	for _, sourceIssue := range sourceIssues {
		issues = append(issues, newIssue(
			unsupportedTrustedStageEditionIssueCode(sourceIssue.Coordinate()),
			"trusted Stage edition "+sourceIssue.Coordinate().String(),
			sourceIssue.Expected(),
			sourceIssue.Actual(),
			sourceIssue.Repair(),
		))
	}
	return normalizeIssues(issues)
}

func unsupportedTrustedStageEditionIssueCode(
	coordinate TrustedStageEditionCoordinate,
) StageRevalidationIssueCode {
	switch coordinate {
	case TrustedStageSchemaEdition:
		return IssueStageSchemaUnsupported
	case TrustedStageCompilerEdition:
		return IssueStageCompilerUnsupported
	case TrustedBaseCompilerSchemaEdition:
		return IssueBaseCompilerUnsupported
	case TrustedStageProducerEdition:
		return IssueStageProducerUnsupported
	case TrustedStageRevalidatorEdition:
		return IssueStageRevalidatorUnsupported
	case TrustedCompositeLowererEdition:
		return IssueCompositeLowererUnsupported
	default:
		return IssueTrustedEditionInputInvalid
	}
}

func compareTargetRuntimeRegistry(
	requirement TargetRuntimeRegistryRequirement,
	registry projecttypeenvruntime.ExactTargetRuntimeRegistry,
) []StageRevalidationIssue {
	if !registry.Valid() {
		return nil
	}
	actual, exists := registry.RuntimeBasisRef()
	if !exists {
		return []StageRevalidationIssue{newIssue(
			IssueTrustedEditionInputInvalid,
			"exact target runtime registry",
			"a valid registry exposing its exact X",
			"runtime-basis coordinate unavailable",
			"rerun current target-runtime observation",
		)}
	}
	expected := requirement.TargetRuntimeBasis()
	if actual == expected {
		return nil
	}
	return []StageRevalidationIssue{newIssue(
		IssueTargetRuntimeBasisMismatch,
		"exact runtime registry for target C",
		expected.String(),
		actual.String(),
		"observe the current process registries against the exact X bound by Stage and executable C",
	)}
}

func collectInvalidInputIssues(
	input ProjectTypeEnvStageRevalidationInput,
) []StageRevalidationIssue {
	issues := make([]StageRevalidationIssue, 0, 6)
	if err := input.Stage.Verify(); err != nil {
		issues = append(issues, newIssue(
			IssueStageInvalid,
			"Stage",
			"an exact immutable ProjectTypeEnvStage",
			err.Error(),
			"reload and verify the exact Stage bytes",
		))
	}
	if err := input.FinalVerification.Verify(); err != nil {
		issues = append(issues, newIssue(
			IssueFinalVerificationInvalid,
			"final verification",
			"a sealed final-lowerer capability",
			err.Error(),
			"rerun final lowering from exact B/E/X/C",
		))
	}
	if err := input.ExecutableTarget.Verify(); err != nil {
		issues = append(issues, newIssue(
			IssueExecutableSnapshotInvalid,
			"executable target",
			"a restored executable snapshot capability",
			err.Error(),
			"restore the exact executable snapshot by replaying final lowering",
		))
	}
	if err := input.CurrentGraph.Verify(); err != nil {
		issues = append(issues, newIssue(
			IssueGraphObservationInvalid,
			"current project graph",
			"an exact graph closure and active-relation observation",
			err.Error(),
			"reread the graph closure and active assertions in the selection transaction",
		))
	}
	if err := verifyCurrentProfileBasis(input.CurrentProfile); err != nil {
		issues = append(issues, newIssue(
			IssueProjectProfileBasisInvalid,
			"current project profile",
			"one exact current project-profile basis",
			err.Error(),
			"reread the canonical project-profile ledger in the selection transaction",
		))
	}
	if err := verifyHeadObservation(input.CurrentHead); err != nil {
		issues = append(issues, newIssue(
			IssueHeadObservationInvalid,
			"current project TypeEnv head",
			"a closed exact head observation",
			err.Error(),
			"reread the dedicated head slot and construct a closed observation",
		))
	}
	return normalizeIssues(issues)
}

func verifyCurrentProfileBasis(
	basis projecttypeenvprofilebasis.CurrentProjectProfileBasis,
) error {
	switch value := basis.(type) {
	case projecttypeenvprofilebasis.NoCanonicalProjectProfile:
		return value.Verify()
	case projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile:
		return value.Verify()
	default:
		return fmt.Errorf(
			"current project-profile basis has unsupported variant %T",
			basis,
		)
	}
}

func compareCurrentGraph(
	input ProjectTypeEnvStageRevalidationInput,
) []StageRevalidationIssue {
	stage := input.Stage
	basis := input.CurrentGraph.GraphSnapshotBasis()
	issues := make([]StageRevalidationIssue, 0, 3)
	issues = appendIfGraphDifferent(
		issues,
		IssueGraphProjectMismatch,
		"current graph project",
		stage.Project().String(),
		basis.Project().String(),
	)
	issues = appendIfGraphDifferent(
		issues,
		IssueGraphSnapshotMismatch,
		"current graph snapshot basis",
		stage.GraphSnapshotBasis().String(),
		basis.Ref().String(),
	)
	issues = appendIfGraphDifferent(
		issues,
		IssueGraphRevisionMismatch,
		"current graph revision",
		fmt.Sprintf("%d", stage.GraphRevision().Value()),
		fmt.Sprintf("%d", basis.GraphRevision().Value()),
	)
	return normalizeIssues(issues)
}

func compareTargetClosure(
	input ProjectTypeEnvStageRevalidationInput,
) []StageRevalidationIssue {
	stage := input.Stage
	verification := input.FinalVerification
	snapshot := input.ExecutableTarget
	record := snapshot.Record()
	issues := make([]StageRevalidationIssue, 0, 12)
	issues = appendIfDifferent(
		issues,
		IssueBaseMismatch,
		"base B",
		stage.Base().String(),
		verification.BaseTypeEnvRef().String(),
	)
	issues = appendIfOrderedExtensionsDiffer(
		issues,
		"Stage/final verification ordered E DAG",
		stage.OrderedExtensions(),
		verification.ExtensionRefs(),
	)
	issues = appendIfDifferent(
		issues,
		IssueRuntimeBasisMismatch,
		"runtime basis X",
		stage.RuntimeBasis().String(),
		verification.RuntimeEvaluationBasisRef().String(),
	)
	issues = appendIfDifferent(
		issues,
		IssueCompositeMismatch,
		"verified composite C",
		stage.VerifiedComposite().String(),
		verification.CompositeRef().String(),
	)
	issues = appendIfDifferent(
		issues,
		IssueFinalVerificationMismatch,
		"final-verification ref",
		stage.CompositeVerificationRef().String(),
		verification.Ref().String(),
	)
	issues = appendIfDifferent(
		issues,
		IssueFinalVerificationMismatch,
		"final-verification digest",
		stage.CompositeVerificationDigest().String(),
		verification.Digest().String(),
	)
	issues = appendIfDifferent(
		issues,
		IssueExecutableSnapshotMismatch,
		"executable snapshot target C",
		stage.VerifiedComposite().String(),
		record.TypeEnvRef().String(),
	)
	issues = appendIfDifferent(
		issues,
		IssueExecutableSnapshotMismatch,
		"executable snapshot B",
		stage.Base().String(),
		record.BaseTypeEnvRef().String(),
	)
	issues = appendIfOrderedExtensionsDiffer(
		issues,
		"Stage/executable snapshot ordered E DAG",
		stage.OrderedExtensions(),
		record.ExtensionRefs(),
	)
	issues = appendIfDifferent(
		issues,
		IssueExecutableSnapshotMismatch,
		"executable snapshot X",
		stage.RuntimeBasis().String(),
		record.RuntimeEvaluationBasisRef().String(),
	)
	issues = appendIfDifferent(
		issues,
		IssueFinalVerificationMismatch,
		"snapshot/final-verification ref",
		verification.Ref().String(),
		record.VerificationRef().String(),
	)
	issues = appendIfDifferent(
		issues,
		IssueLoweredEnvironmentMismatch,
		"lowered environment ref",
		verification.LoweredEnvironmentRef().String(),
		snapshot.TypeEnvRef().String(),
	)
	issues = appendIfDifferent(
		issues,
		IssueLoweredEnvironmentMismatch,
		"lowered environment digest",
		verification.LoweredEnvironmentDigest().String(),
		snapshot.LoweredEnvironmentDigest().String(),
	)
	issues = appendIfDifferent(
		issues,
		IssueLowererEditionMismatch,
		"lowerer schema edition",
		verification.LowererSchemaVersion(),
		record.LowererSchemaVersion(),
	)
	return normalizeIssues(issues)
}

func comparePredecessor(
	input ProjectTypeEnvStageRevalidationInput,
) []StageRevalidationIssue {
	switch predecessor := input.Stage.Predecessor().(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		return compareGenesisPredecessor(input.Stage, input.CurrentHead)
	case projecttypeenvselection.TransitionStagePredecessor:
		return compareTransitionPredecessor(predecessor, input.CurrentHead)
	default:
		return []StageRevalidationIssue{newIssue(
			IssueHeadPresenceMismatch,
			"Stage predecessor",
			"Genesis or Transition",
			fmt.Sprintf("%T", input.Stage.Predecessor()),
			"rebuild the Stage with one closed predecessor variant",
		)}
	}
}

func compareGenesisPredecessor(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	observation CurrentProjectTypeEnvHeadObservation,
) []StageRevalidationIssue {
	issues := compareHeadProject(stage.Project().String(), observation)
	switch observation.(type) {
	case ObservedNoProjectTypeEnvHead:
		return normalizeIssues(issues)
	case ObservedProjectTypeEnvHead:
		issues = append(issues, newIssue(
			IssueHeadPresenceMismatch,
			"Genesis current head",
			"absent",
			"present",
			"stage a Transition against the exact current head",
		))
		return normalizeIssues(issues)
	default:
		return normalizeIssues(issues)
	}
}

func compareTransitionPredecessor(
	predecessor projecttypeenvselection.TransitionStagePredecessor,
	observation CurrentProjectTypeEnvHeadObservation,
) []StageRevalidationIssue {
	issues := compareHeadProject(predecessor.Project().String(), observation)
	switch current := observation.(type) {
	case ObservedNoProjectTypeEnvHead:
		issues = append(issues, newIssue(
			IssueHeadPresenceMismatch,
			"Transition current head",
			"present",
			"absent",
			"rebuild the Stage against the observed head posture",
		))
	case ObservedProjectTypeEnvHead:
		state := current.State()
		issues = appendIfPredecessorDifferent(
			issues,
			IssuePriorHeadRefMismatch,
			"prior head ref",
			predecessor.Head().String(),
			state.Ref().String(),
		)
		issues = appendIfPredecessorDifferent(
			issues,
			IssuePriorHeadRevisionMismatch,
			"prior HeadRevision",
			fmt.Sprintf("%d", predecessor.HeadRevision().Value()),
			fmt.Sprintf("%d", state.Revision().Value()),
		)
		issues = appendIfPredecessorDifferent(
			issues,
			IssuePriorSelectedCompositeDrift,
			"prior selected C",
			predecessor.SelectedComposite().String(),
			state.SelectedComposite().String(),
		)
	}
	return normalizeIssues(issues)
}

func compareHeadProject(
	expected string,
	observation CurrentProjectTypeEnvHeadObservation,
) []StageRevalidationIssue {
	actual := observation.Project().String()
	if expected == actual {
		return nil
	}
	return []StageRevalidationIssue{newIssue(
		IssueHeadProjectMismatch,
		"current head project",
		expected,
		actual,
		"reread the head slot for the Stage project",
	)}
}

func appendIfDifferent(
	issues []StageRevalidationIssue,
	code StageRevalidationIssueCode,
	subject string,
	expected string,
	actual string,
) []StageRevalidationIssue {
	if expected == actual {
		return issues
	}
	return append(issues, newIssue(
		code,
		subject,
		expected,
		actual,
		"reload the exact target closure bound by Stage",
	))
}

func appendIfPredecessorDifferent(
	issues []StageRevalidationIssue,
	code StageRevalidationIssueCode,
	subject string,
	expected string,
	actual string,
) []StageRevalidationIssue {
	if expected == actual {
		return issues
	}
	return append(issues, newIssue(
		code,
		subject,
		expected,
		actual,
		"rebuild the Stage against the exact current head",
	))
}

func appendIfGraphDifferent(
	issues []StageRevalidationIssue,
	code StageRevalidationIssueCode,
	subject string,
	expected string,
	actual string,
) []StageRevalidationIssue {
	if expected == actual {
		return issues
	}
	return append(issues, newIssue(
		code,
		subject,
		expected,
		actual,
		"rebuild the Stage against the exact current project graph",
	))
}

func appendIfOrderedExtensionsDiffer(
	issues []StageRevalidationIssue,
	subject string,
	expected []typedmemory.TypeEnvExtensionRef,
	actual []typedmemory.TypeEnvExtensionRef,
) []StageRevalidationIssue {
	if orderedExtensionsEqual(expected, actual) {
		return issues
	}
	return append(issues, newIssue(
		IssueOrderedExtensionsMismatch,
		subject,
		formatExtensionRefs(expected),
		formatExtensionRefs(actual),
		"reload the exact ordered E DAG bound by Stage",
	))
}

func orderedExtensionsEqual(
	expected []typedmemory.TypeEnvExtensionRef,
	actual []typedmemory.TypeEnvExtensionRef,
) bool {
	if len(expected) != len(actual) {
		return false
	}
	for index := range expected {
		if expected[index] != actual[index] {
			return false
		}
	}
	return true
}

func formatExtensionRefs(values []typedmemory.TypeEnvExtensionRef) string {
	result := "["
	for index, value := range values {
		if index > 0 {
			result += ","
		}
		result += value.String()
	}
	return result + "]"
}
