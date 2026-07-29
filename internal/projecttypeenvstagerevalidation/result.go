package projecttypeenvstagerevalidation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var ErrCurrentSelectionStageNotSerializable = errors.New(
	"CurrentSelectionStage is an in-process capability and cannot be serialized",
)

// StageRevalidationResult is a closed result algebra.
type StageRevalidationResult interface {
	stageRevalidationResultVariant()
}

type currentSelectionStageCapability struct{}

type currentSelectionStageState struct {
	stage        projecttypeenvselection.ProjectTypeEnvStage
	assertions   projecttypeenvassertionreport.Report
	profileBasis projecttypeenvprofilebasis.CurrentProjectProfileBasis
	profile      projecttypeenvprofilefit.Assessment
}

// CurrentSelectionStage is an opaque, non-authorizing, non-serializable
// capability minted only after exact current Stage revalidation. It proves no
// authority and cannot move a project TypeEnv head by itself.
type CurrentSelectionStage struct {
	state      *currentSelectionStageState
	capability *currentSelectionStageCapability
}

func (CurrentSelectionStage) stageRevalidationResultVariant() {}

func (value CurrentSelectionStage) Valid() bool {
	if value.state == nil || value.capability == nil {
		return false
	}
	if err := value.state.stage.Verify(); err != nil {
		return false
	}
	if err := value.state.assertions.Verify(); err != nil {
		return false
	}
	if err := verifyCurrentProfileBasis(value.state.profileBasis); err != nil {
		return false
	}
	if value.state.profile == nil {
		return false
	}
	if err := value.state.profile.Verify(); err != nil {
		return false
	}
	if value.state.assertions.Posture() != typedmemory.RevalidationClean {
		return false
	}
	if _, compatible := value.state.profile.(projecttypeenvprofilefit.Compatible); !compatible {
		return false
	}
	if value.state.stage.ProfileLedgerRevision() !=
		value.state.profileBasis.LedgerRevision() {
		return false
	}
	if value.state.stage.ProfileLedgerDigest() !=
		value.state.profileBasis.ProfileLedgerDigest() {
		return false
	}
	if !transitionProjectionProfilesSelectionReady(value.state.stage) {
		return false
	}
	if value.state.profile.BasisRef() !=
		value.state.profileBasis.ProfileBasisRef() {
		return false
	}
	if value.state.profile.BasisDigest() != value.state.profileBasis.Digest() {
		return false
	}
	stageAssertions := value.state.stage.ExistingAssertionRevalidation()
	stageProfile := value.state.stage.ProfileCompatibility()
	return bytes.Equal(
		stageAssertions.CanonicalBytes(),
		value.state.assertions.CanonicalBytes(),
	) && bytes.Equal(
		stageProfile.CanonicalBytes(),
		value.state.profile.CanonicalBytes(),
	)
}

func transitionProjectionProfilesSelectionReady(
	stage projecttypeenvselection.ProjectTypeEnvStage,
) bool {
	if _, transition := stage.Predecessor().(projecttypeenvselection.TransitionStagePredecessor); !transition {
		return true
	}
	artifact, exists := stage.TransitionProjectionProfileCompatibility()
	if !exists {
		return false
	}
	blocked, err := projecttypeenvprofilecompatibility.TransitionProjectionProfilesHaveBlockedProfile(
		artifact,
	)
	return err == nil && !blocked
}

func (value CurrentSelectionStage) Stage() (
	projecttypeenvselection.ProjectTypeEnvStage,
	bool,
) {
	if !value.Valid() {
		return projecttypeenvselection.ProjectTypeEnvStage{}, false
	}
	return value.state.stage, true
}

func (value CurrentSelectionStage) AssertionRevalidation() (
	projecttypeenvassertionreport.Report,
	bool,
) {
	if !value.Valid() {
		return projecttypeenvassertionreport.Report{}, false
	}
	return value.state.assertions, true
}

func (value CurrentSelectionStage) ProfileAssessment() (
	projecttypeenvprofilefit.Assessment,
	bool,
) {
	if !value.Valid() {
		return nil, false
	}
	return value.state.profile, true
}

func (value CurrentSelectionStage) ProfileBasis() (
	projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	bool,
) {
	if !value.Valid() {
		return nil, false
	}
	return value.state.profileBasis, true
}

func newCurrentSelectionStage(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	assertions projecttypeenvassertionreport.Report,
	profileBasis projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	profile projecttypeenvprofilefit.Assessment,
) (CurrentSelectionStage, error) {
	if err := verifyCurrentProfileBasis(profileBasis); err != nil {
		return CurrentSelectionStage{}, fmt.Errorf(
			"verify current project-profile basis: %w",
			err,
		)
	}
	ownedAssertions, err := projecttypeenvassertionreport.DecodeCanonicalReport(
		assertions.CanonicalBytes(),
	)
	if err != nil {
		return CurrentSelectionStage{}, fmt.Errorf(
			"copy current assertion revalidation: %w",
			err,
		)
	}
	ownedProfile, err := projecttypeenvprofilefit.DecodeCanonicalAssessment(
		profile.CanonicalBytes(),
	)
	if err != nil {
		return CurrentSelectionStage{}, fmt.Errorf(
			"copy current profile assessment: %w",
			err,
		)
	}
	value := CurrentSelectionStage{
		state: &currentSelectionStageState{
			stage:        stage,
			assertions:   ownedAssertions,
			profileBasis: profileBasis,
			profile:      ownedProfile,
		},
		capability: &currentSelectionStageCapability{},
	}
	if !value.Valid() {
		return CurrentSelectionStage{}, fmt.Errorf(
			"current selection Stage inputs are not selection-ready",
		)
	}
	return value, nil
}

func (CurrentSelectionStage) MarshalJSON() ([]byte, error) {
	return nil, ErrCurrentSelectionStageNotSerializable
}

func (*CurrentSelectionStage) UnmarshalJSON([]byte) error {
	return ErrCurrentSelectionStageNotSerializable
}

var (
	_ json.Marshaler   = CurrentSelectionStage{}
	_ json.Unmarshaler = (*CurrentSelectionStage)(nil)
)

// InvalidSelectionStage means one supposedly sealed input failed its own
// integrity check. It is distinct from semantic drift and unavailable derived
// inputs.
type InvalidSelectionStage struct {
	issues []StageRevalidationIssue
}

func (InvalidSelectionStage) stageRevalidationResultVariant() {}

func (result InvalidSelectionStage) Issues() []StageRevalidationIssue {
	return append([]StageRevalidationIssue(nil), result.issues...)
}

// DriftedSelectionStage means all input values were structurally valid, but
// at least one current mutable basis no longer equals the Stage: target
// closure, predecessor, trusted editions/runtime, graph snapshot, assertion
// report, or project-profile ledger/fit identity.
type DriftedSelectionStage struct {
	issues []StageRevalidationIssue
}

func (DriftedSelectionStage) stageRevalidationResultVariant() {}

func (result DriftedSelectionStage) Issues() []StageRevalidationIssue {
	return append([]StageRevalidationIssue(nil), result.issues...)
}

// RejectedSelectionStage means every current basis exactly matches the Stage,
// but its present semantic posture is not selection-ready. Restaging would
// reproduce the same rejection; the typed issues name the missing or
// contradicted grounds that must change.
type RejectedSelectionStage struct {
	issues []StageRevalidationIssue
}

func (RejectedSelectionStage) stageRevalidationResultVariant() {}

func (result RejectedSelectionStage) Issues() []StageRevalidationIssue {
	return append([]StageRevalidationIssue(nil), result.issues...)
}

// DerivedInputRequirement names one semantic derivation that the current
// revalidator refuses to accept as caller-supplied truth. Historical enum
// values remain stable even after a package-owned producer removes them from
// current Unavailable results.
type DerivedInputRequirement uint8

const (
	RequirementCurrentGraphBasis DerivedInputRequirement = iota + 1
	RequirementTargetRuntimeRegistry
	RequirementExistingAssertionRevalidation
	RequirementCurrentProjectProfileFit
	RequirementTypeEnvCompatibility
	RequirementTrustedStageEditions
)

func (requirement DerivedInputRequirement) String() string {
	switch requirement {
	case RequirementCurrentGraphBasis:
		return "current_graph_basis"
	case RequirementTargetRuntimeRegistry:
		return "target_runtime_registry"
	case RequirementExistingAssertionRevalidation:
		return "existing_assertion_revalidation"
	case RequirementCurrentProjectProfileFit:
		return "current_project_profile_fit"
	case RequirementTypeEnvCompatibility:
		return "typeenv_compatibility"
	case RequirementTrustedStageEditions:
		return "trusted_stage_editions"
	default:
		return ""
	}
}

// UnavailableSelectionStage means the implemented exact comparisons passed,
// but one or more required transaction-local producers are unavailable. In
// v1 this is limited to an absent target-runtime observation or unavailable
// prior selected executable TypeEnv needed for Transition compatibility.
type UnavailableSelectionStage struct {
	requirements []DerivedInputRequirement
}

func (UnavailableSelectionStage) stageRevalidationResultVariant() {}

func (result UnavailableSelectionStage) Requirements() []DerivedInputRequirement {
	return append([]DerivedInputRequirement(nil), result.requirements...)
}

func newInvalidResult(issues []StageRevalidationIssue) InvalidSelectionStage {
	return InvalidSelectionStage{issues: normalizeIssues(issues)}
}

func newDriftedResult(issues []StageRevalidationIssue) DriftedSelectionStage {
	return DriftedSelectionStage{issues: normalizeIssues(issues)}
}

func newRejectedResult(issues []StageRevalidationIssue) RejectedSelectionStage {
	return RejectedSelectionStage{issues: normalizeIssues(issues)}
}

func newUnavailableResult(
	requirements []DerivedInputRequirement,
) UnavailableSelectionStage {
	owned := append([]DerivedInputRequirement(nil), requirements...)
	sort.Slice(owned, func(left int, right int) bool {
		return owned[left] < owned[right]
	})
	result := make([]DerivedInputRequirement, 0, len(owned))
	for _, requirement := range owned {
		if len(result) > 0 && result[len(result)-1] == requirement {
			continue
		}
		result = append(result, requirement)
	}
	return UnavailableSelectionStage{requirements: result}
}
