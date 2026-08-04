package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	CompatibleSuccessorAuthorityGeneration = "compatible_successor_policy"
	CompatibleSuccessorResolutionKind      = "automatic_compatible_successor"
	CompatibleSuccessorPolicyEdition       = "haft.project-typeenv.compatible-successor-policy/v1"

	compatibleSuccessorResolutionSchema = "haft.project-typeenv.compatible-successor-resolution/v1"
	compatibleSuccessorResolutionDomain = "haft.project-typeenv.compatible-successor-resolution/v1"
	compatibleSuccessorPolicyDomain     = "haft.project-typeenv.compatible-successor-policy/v1"
	maximumCompatibleSuccessorBytes     = 2 << 20
)

// CompatibleSuccessorResolution is the exact system-policy satisfaction
// record for one automatic post-Genesis transition. It is not an operator
// request, SpeechAct, approval, or reusable capability. The effect shell must
// rebuild it from the transaction-current Stage and project binding before it
// can mint a one-shot authority use.
type CompatibleSuccessorResolution struct {
	ref            ProjectTypeEnvHeadSelectionAuthorityResolutionRef
	digest         authority.Digest
	request        projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	content        ProjectTypeEnvHeadSelectionAuthorizationContent
	stage          projecttypeenvselection.ProjectTypeEnvStage
	projectBinding ProjectAuthorityContextBinding
	policyDigest   authority.Digest
	evaluatedAt    time.Time
	canonicalJSON  []byte
}

type CompatibleSuccessorResolutionInput struct {
	Request        projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	Content        ProjectTypeEnvHeadSelectionAuthorizationContent
	Stage          projecttypeenvselection.ProjectTypeEnvStage
	ProjectBinding ProjectAuthorityContextBinding
	EvaluatedAt    time.Time
}

type compatibleSuccessorResolutionProjection struct {
	Schema                                string `json:"schema"`
	AuthorityGeneration                   string `json:"authority_generation"`
	ResolutionKind                        string `json:"resolution_kind"`
	PolicyEdition                         string `json:"policy_edition"`
	PolicyDigest                          string `json:"policy_digest"`
	Project                               string `json:"project_id"`
	ProjectRoot                           string `json:"project_root"`
	ProjectBindingDigest                  string `json:"project_binding_digest"`
	RequestRef                            string `json:"request_ref"`
	RequestDigest                         string `json:"request_digest"`
	ContentRef                            string `json:"content_ref"`
	ContentDigest                         string `json:"content_digest"`
	StageRef                              string `json:"stage_ref"`
	StageDigest                           string `json:"stage_digest"`
	CompatibilityRef                      string `json:"compatibility_ref"`
	CompatibilityDigest                   string `json:"compatibility_digest"`
	AssertionRevalidationRef              string `json:"assertion_revalidation_ref"`
	AssertionRevalidationDigest           string `json:"assertion_revalidation_digest"`
	ProjectProfileFitRef                  string `json:"project_profile_fit_ref"`
	ProjectProfileFitDigest               string `json:"project_profile_fit_digest"`
	ProjectionProfileCompatibilityRef     string `json:"projection_profile_compatibility_ref"`
	ProjectionProfileCompatibilityDigest  string `json:"projection_profile_compatibility_digest"`
	ProjectionProfileCompatibilityPosture string `json:"projection_profile_compatibility_posture"`
	ExistingAssertionRevalidationPosture  string `json:"existing_assertion_revalidation_posture"`
	ProjectProfileCompatibilityPosture    string `json:"project_profile_compatibility_posture"`
	PredicateResult                       string `json:"predicate_result"`
	EvaluatedAt                           string `json:"evaluated_at"`
	CurrentnessBoundary                   string `json:"currentness_boundary"`
}

func SealCompatibleSuccessorResolution(
	input CompatibleSuccessorResolutionInput,
) (CompatibleSuccessorResolution, error) {
	if err := input.Request.Verify(); err != nil {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor request: %w",
			err,
		)
	}
	if _, ok := input.Request.Predecessor().(projecttypeenvselection.TransitionStagePredecessor); !ok {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor policy requires an exact Transition predecessor",
		)
	}
	if err := input.Content.ExactAgainst(input.Request); err != nil {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor content: %w",
			err,
		)
	}
	if err := input.Stage.Verify(); err != nil {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor Stage: %w",
			err,
		)
	}
	if input.Stage.Project() != input.Request.Project() ||
		input.Stage.Ref() != input.Request.Target().Stage() ||
		input.Stage.VerifiedComposite() != input.Request.Target().VerifiedComposite() {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor Stage differs from the exact selection request",
		)
	}
	if !input.ProjectBinding.ExactFor(
		input.Request.Project(),
		input.ProjectBinding.Root(),
		input.Content.JudgementContext(),
	) {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor project binding is not exact",
		)
	}
	evaluatedAt := input.EvaluatedAt.Round(0).UTC()
	if evaluatedAt.IsZero() || !input.Content.ValidityWindow().Contains(evaluatedAt) {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor evaluation is outside the exact content validity window",
		)
	}
	revalidation := input.Stage.ExistingAssertionRevalidation()
	if revalidation.Posture() != typedmemory.RevalidationClean {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor existing assertion revalidation is %s",
			revalidation.Posture().String(),
		)
	}
	profileFit := input.Stage.ProfileCompatibility()
	if _, ok := profileFit.(projecttypeenvprofilefit.Compatible); !ok {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor project profile posture is not compatible",
		)
	}
	transitionProfiles, present :=
		input.Stage.TransitionProjectionProfileCompatibility()
	if !present {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor Stage has no projection-profile compatibility artifact",
		)
	}
	profiles, err :=
		projecttypeenvprofilecompatibility.DecodeTransitionProjectionProfiles(
			transitionProfiles,
		)
	if err != nil {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor projection profiles: %w",
			err,
		)
	}
	if profiles.HasBlockedProfile() {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor projection profile is blocked",
		)
	}
	policyDigest, err := compatibleSuccessorPolicyDigest()
	if err != nil {
		return CompatibleSuccessorResolution{}, err
	}
	transitionProfilesRef, _ :=
		input.Stage.TransitionProjectionProfileCompatibilityRef()
	transitionProfilesDigest, _ :=
		input.Stage.TransitionProjectionProfileCompatibilityDigest()
	projection := compatibleSuccessorResolutionProjection{
		Schema:                                compatibleSuccessorResolutionSchema,
		AuthorityGeneration:                   CompatibleSuccessorAuthorityGeneration,
		ResolutionKind:                        CompatibleSuccessorResolutionKind,
		PolicyEdition:                         CompatibleSuccessorPolicyEdition,
		PolicyDigest:                          policyDigest.String(),
		Project:                               input.Request.Project().String(),
		ProjectRoot:                           input.ProjectBinding.Root().String(),
		ProjectBindingDigest:                  input.ProjectBinding.Digest().String(),
		RequestRef:                            input.Request.Ref().String(),
		RequestDigest:                         input.Request.Ref().Digest().String(),
		ContentRef:                            input.Content.DescriptionRef().String(),
		ContentDigest:                         input.Content.Digest().String(),
		StageRef:                              input.Stage.Ref().String(),
		StageDigest:                           input.Stage.Ref().Digest().String(),
		CompatibilityRef:                      input.Stage.CompatibilityRef().String(),
		CompatibilityDigest:                   input.Stage.CompatibilityDigest().String(),
		AssertionRevalidationRef:              input.Stage.ExistingAssertionRevalidationRef().String(),
		AssertionRevalidationDigest:           input.Stage.ExistingAssertionRevalidationDigest().String(),
		ProjectProfileFitRef:                  input.Stage.ProfileFitRef().String(),
		ProjectProfileFitDigest:               input.Stage.ProfileFitDigest().String(),
		ProjectionProfileCompatibilityRef:     transitionProfilesRef.String(),
		ProjectionProfileCompatibilityDigest:  transitionProfilesDigest.String(),
		ProjectionProfileCompatibilityPosture: "compatible_or_degraded_no_blocked_profile",
		ExistingAssertionRevalidationPosture:  typedmemory.RevalidationClean.String(),
		ProjectProfileCompatibilityPosture:    "compatible",
		PredicateResult:                       "satisfied",
		EvaluatedAt:                           evaluatedAt.Format(time.RFC3339Nano),
		CurrentnessBoundary:                   "effect_shell_rebuilds_from_transaction_current_stage_project_binding_head_graph_profile_and_runtime_before_cas",
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return CompatibleSuccessorResolution{}, err
	}
	if len(canonical) > maximumCompatibleSuccessorBytes {
		return CompatibleSuccessorResolution{}, fmt.Errorf(
			"compatible-successor resolution exceeds %d bytes",
			maximumCompatibleSuccessorBytes,
		)
	}
	digest, err := digestCanonical(compatibleSuccessorResolutionDomain, canonical)
	if err != nil {
		return CompatibleSuccessorResolution{}, err
	}
	return CompatibleSuccessorResolution{
		ref:            ProjectTypeEnvHeadSelectionAuthorityResolutionRef{digest: digest},
		digest:         digest,
		request:        input.Request,
		content:        input.Content,
		stage:          input.Stage,
		projectBinding: input.ProjectBinding,
		policyDigest:   policyDigest,
		evaluatedAt:    evaluatedAt,
		canonicalJSON:  canonical,
	}, nil
}

func compatibleSuccessorPolicyDigest() (authority.Digest, error) {
	canonical, err := json.Marshal(struct {
		Edition   string `json:"edition"`
		Predicate string `json:"predicate"`
	}{
		Edition:   CompatibleSuccessorPolicyEdition,
		Predicate: "transition_and_exact_current_stage_and_clean_assertions_and_compatible_project_profile_and_no_blocked_projection_profile",
	})
	if err != nil {
		return authority.Digest{}, err
	}
	return digestCanonical(compatibleSuccessorPolicyDomain, canonical)
}

// CompatibleSuccessorPolicyDigest returns the immutable identity of the
// package-owned automatic-transition predicate.
func CompatibleSuccessorPolicyDigest() (authority.Digest, error) {
	return compatibleSuccessorPolicyDigest()
}

func (value CompatibleSuccessorResolution) Verify() error {
	rebuilt, err := SealCompatibleSuccessorResolution(
		CompatibleSuccessorResolutionInput{
			Request:        value.request,
			Content:        value.content,
			Stage:          value.stage,
			ProjectBinding: value.projectBinding,
			EvaluatedAt:    value.evaluatedAt,
		},
	)
	if err != nil {
		return err
	}
	if rebuilt.ref != value.ref || rebuilt.digest != value.digest ||
		rebuilt.policyDigest != value.policyDigest ||
		!bytes.Equal(rebuilt.canonicalJSON, value.canonicalJSON) {
		return fmt.Errorf("compatible-successor resolution is not exact")
	}
	return nil
}

func (value CompatibleSuccessorResolution) Ref() ProjectTypeEnvHeadSelectionAuthorityResolutionRef {
	return value.ref
}

func (value CompatibleSuccessorResolution) Digest() authority.Digest { return value.digest }

func (value CompatibleSuccessorResolution) Request() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest {
	return value.request
}

func (value CompatibleSuccessorResolution) Content() ProjectTypeEnvHeadSelectionAuthorizationContent {
	return value.content
}

func (value CompatibleSuccessorResolution) Stage() projecttypeenvselection.ProjectTypeEnvStage {
	return value.stage
}

func (value CompatibleSuccessorResolution) ProjectBinding() ProjectAuthorityContextBinding {
	return value.projectBinding
}

func (value CompatibleSuccessorResolution) PolicyDigest() authority.Digest {
	return value.policyDigest
}

func (value CompatibleSuccessorResolution) EvaluatedAt() time.Time { return value.evaluatedAt }

func (value CompatibleSuccessorResolution) CanonicalJSON() []byte {
	return append([]byte(nil), value.canonicalJSON...)
}
