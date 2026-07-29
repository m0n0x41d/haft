package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
)

const (
	authorityResolutionBasisSchema       = "haft.project-typeenv.head-selection-authority-resolution-basis/v1"
	authorityResolutionBasisDomain       = "haft.project-typeenv.head-selection-authority-resolution-basis/v1"
	maximumAuthorityResolutionBasisBytes = 512 * 1024
)

type AuthorityRejectionCode string

const (
	AuthorityRejectedInvalidBasis       AuthorityRejectionCode = "invalid_basis"
	AuthorityRejectedExpired            AuthorityRejectionCode = "outside_validity_window"
	AuthorityRejectedBeforeSpeechAct    AuthorityRejectionCode = "before_speech_act_completion"
	AuthorityRejectedContextMismatch    AuthorityRejectionCode = "context_mismatch"
	AuthorityRejectedPolicyMismatch     AuthorityRejectionCode = "resolver_policy_mismatch"
	AuthorityRejectedActTypeMismatch    AuthorityRejectionCode = "act_type_mismatch"
	AuthorityRejectedAssignmentMismatch AuthorityRejectionCode = "role_assignment_mismatch"
	AuthorityRejectedAdapterMismatch    AuthorityRejectionCode = "source_adapter_mismatch"
)

type AuthorityRejection struct {
	code   AuthorityRejectionCode
	detail string
}

func (rejection AuthorityRejection) Error() string {
	return string(rejection.code) + ": " + rejection.detail
}

func (rejection AuthorityRejection) Code() AuthorityRejectionCode { return rejection.code }

// ProjectTypeEnvHeadSelectionAuthorityResolutionBasis is the sealed result of
// checking one occurrence against one exact reusable resolver-policy snapshot
// at one effect-shell supplied instant. It is not a claim that this policy is
// still the registry head and it says nothing about the TypeEnv head CAS.
type ProjectTypeEnvHeadSelectionAuthorityResolutionBasis struct {
	ref           ProjectTypeEnvHeadSelectionAuthorityResolutionBasisRef
	digest        authority.Digest
	policy        ProjectTypeEnvHeadSelectionResolverPolicy
	record        ProjectTypeEnvHeadSelectionSpeechActRecord
	content       ProjectTypeEnvHeadSelectionAuthorizationContent
	request       projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	stage         projecttypeenvselection.ProjectTypeEnvStage
	evaluatedAt   time.Time
	canonicalJSON []byte
}

type ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput struct {
	Policy      ProjectTypeEnvHeadSelectionResolverPolicy
	Record      ProjectTypeEnvHeadSelectionSpeechActRecord
	Content     ProjectTypeEnvHeadSelectionAuthorizationContent
	Request     projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	Stage       projecttypeenvselection.ProjectTypeEnvStage
	EvaluatedAt time.Time
}

type authorityResolutionBasisProjection struct {
	Schema                 string                `json:"schema"`
	EvaluatedAt            string                `json:"evaluated_at"`
	ResolverPolicyRef      string                `json:"resolver_policy_ref"`
	ResolverPolicyEdition  string                `json:"resolver_policy_edition"`
	ResolverPolicyDigest   string                `json:"resolver_policy_digest"`
	SourceContractDigest   string                `json:"speech_act_source_contract_digest"`
	SourceAdapterDigest    string                `json:"speech_act_source_adapter_policy_digest"`
	ProjectBindingDigest   string                `json:"project_context_binding_digest"`
	SpeechActRef           string                `json:"speech_act_ref"`
	WorkRef                string                `json:"speech_act_work_ref"`
	SpeechActSourceDigest  string                `json:"speech_act_source_digest"`
	SpeechActRecordRef     string                `json:"speech_act_record_ref"`
	SpeechActRecordDigest  string                `json:"speech_act_record_digest"`
	ContentDescriptionKind string                `json:"content_description_ref_kind"`
	ContentDescriptionRef  string                `json:"content_description_ref"`
	ContentDigest          string                `json:"content_digest"`
	PermissionRef          string                `json:"permission_ref"`
	PermissionRecordDigest string                `json:"permission_record_digest"`
	RequestRef             string                `json:"request_ref"`
	RequestDigest          string                `json:"request_digest"`
	Project                string                `json:"project_id"`
	ProjectRoot            string                `json:"project_root"`
	Head                   string                `json:"head_ref"`
	Predecessor            predecessorProjection `json:"predecessor"`
	Action                 string                `json:"action"`
	JudgementContext       string                `json:"judgement_context_ref"`
	ContextPolicyRef       string                `json:"speech_act_context_policy_ref"`
	ContextPolicyDigest    string                `json:"speech_act_context_policy_digest"`
	RoleAssignmentRef      string                `json:"role_assignment_ref"`
	RoleAssignmentDigest   string                `json:"role_assignment_digest"`
	HolderSystemRef        string                `json:"holder_system_ref"`
	RoleRef                string                `json:"role_ref"`
	ValidityFrom           string                `json:"validity_from"`
	ValidityUntil          string                `json:"validity_until"`
	Stage                  string                `json:"stage_ref"`
	VerifiedComposite      string                `json:"verified_composite_ref"`
	ExpectedGraphRevision  uint64                `json:"expected_graph_revision"`
	IdempotencyKey         string                `json:"idempotency_key"`
	CurrentnessBoundary    string                `json:"currentness_boundary"`
}

func SealProjectTypeEnvHeadSelectionAuthorityResolutionBasis(
	input ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput,
) (ProjectTypeEnvHeadSelectionAuthorityResolutionBasis, error) {
	evaluatedAt, err := evaluateAuthorityResolutionBasis(input)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityResolutionBasis{}, err
	}
	basis := ProjectTypeEnvHeadSelectionAuthorityResolutionBasis{
		policy:      input.Policy,
		record:      input.Record,
		content:     input.Content,
		request:     input.Request,
		stage:       input.Stage,
		evaluatedAt: evaluatedAt,
	}
	projection, err := projectAuthorityResolutionBasis(basis)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityResolutionBasis{}, err
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityResolutionBasis{}, err
	}
	if len(canonical) > maximumAuthorityResolutionBasisBytes {
		return ProjectTypeEnvHeadSelectionAuthorityResolutionBasis{}, fmt.Errorf(
			"TypeEnv authority resolution basis exceeds %d bytes",
			maximumAuthorityResolutionBasisBytes,
		)
	}
	digest, err := digestCanonical(authorityResolutionBasisDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityResolutionBasis{}, err
	}
	basis.digest = digest
	basis.ref = ProjectTypeEnvHeadSelectionAuthorityResolutionBasisRef{digest: digest}
	basis.canonicalJSON = canonical
	return basis, nil
}

func evaluateAuthorityResolutionBasis(
	input ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput,
) (time.Time, error) {
	policy := input.Policy
	contract := input.Record.SourceContract()
	binding := input.Record.ProjectBinding()
	adapter := policy.SourceAdapter()
	if err := policy.ExactAgainst(contract, adapter, binding); err != nil {
		return time.Time{}, reject(AuthorityRejectedPolicyMismatch, err.Error())
	}
	if policy.Action() != input.Content.Action() {
		return time.Time{}, reject(
			AuthorityRejectedPolicyMismatch,
			"resolver policy admitted action differs from reviewed content action",
		)
	}
	if err := input.Content.ExactAgainst(input.Request); err != nil {
		return time.Time{}, reject(AuthorityRejectedInvalidBasis, err.Error())
	}
	if !sameStage(input.Content.Stage(), input.Stage) {
		return time.Time{}, reject(
			AuthorityRejectedInvalidBasis,
			"Stage differs from reviewed content",
		)
	}
	if err := input.Record.Verify(input.Request); err != nil {
		return time.Time{}, reject(AuthorityRejectedInvalidBasis, err.Error())
	}
	if input.Record.Digest() != input.Record.Ref().Digest() {
		return time.Time{}, reject(
			AuthorityRejectedInvalidBasis,
			"SpeechAct-record ref/digest mismatch",
		)
	}
	recordContent := input.Record.Content()
	contentMatches := recordContent.Digest() == input.Content.Digest() &&
		recordContent.DescriptionRef() == input.Content.DescriptionRef()
	if !contentMatches {
		return time.Time{}, reject(
			AuthorityRejectedInvalidBasis,
			"SpeechAct record content mismatch",
		)
	}
	evaluatedAt := input.EvaluatedAt.Round(0).UTC()
	if evaluatedAt.IsZero() || !input.Content.ValidityWindow().Contains(evaluatedAt) {
		return time.Time{}, reject(
			AuthorityRejectedExpired,
			"evaluation instant is outside reviewed validity",
		)
	}
	if !policy.EffectiveWindow().Contains(evaluatedAt) {
		return time.Time{}, reject(
			AuthorityRejectedPolicyMismatch,
			"evaluation instant is outside exact resolver-policy window",
		)
	}
	source := input.Record.Source()
	workWindow, workOK := source.WorkWindow()
	if !workOK || evaluatedAt.Before(workWindow.Until()) {
		return time.Time{}, reject(
			AuthorityRejectedBeforeSpeechAct,
			"evaluation precedes SpeechAct Work completion",
		)
	}
	if err := adapter.VerifySource(source); err != nil {
		return time.Time{}, reject(AuthorityRejectedAdapterMismatch, err.Error())
	}
	if err := evaluateSourcePolicy(source, input.Content); err != nil {
		return time.Time{}, err
	}
	return evaluatedAt, nil
}

func evaluateSourcePolicy(
	source authority.VerifiedSpeechActSourceV2,
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) error {
	context, contextOK := source.BoundedContext()
	policy, policyOK := source.ContextPolicy()
	policyContext, policyContextOK := policy.BoundedContext()
	policyActType, policyActTypeOK := policy.RecognizedActType()
	actTypes, actTypesOK := source.ActTypeRefs()
	if !contextOK || !policyOK || !policyContextOK || !policyActTypeOK || !actTypesOK {
		return reject(AuthorityRejectedInvalidBasis, "SpeechAct policy coordinates are unavailable")
	}
	if context != content.JudgementContext() || policyContext != content.JudgementContext() {
		return reject(AuthorityRejectedContextMismatch, "SpeechAct and content contexts differ")
	}
	if len(actTypes) != 1 || actTypes[0] != policyActType {
		return reject(AuthorityRejectedActTypeMismatch, "SpeechAct actType is not recognized by source policy")
	}
	assignment, assignmentOK := source.PerformedByRoleAssignment()
	assignmentContext, assignmentContextOK := assignment.BoundedContext()
	assignmentWindow, assignmentWindowOK := assignment.AssignmentWindow()
	workWindow, workWindowOK := source.WorkWindow()
	_, assignmentRefOK := assignment.Ref()
	_, assignmentDigestOK := assignment.Digest()
	_, holderOK := assignment.HolderSystemRef()
	_, roleOK := assignment.RoleRef()
	present := assignmentOK && assignmentContextOK && assignmentWindowOK &&
		workWindowOK && assignmentRefOK && assignmentDigestOK && holderOK && roleOK
	if !present {
		return reject(AuthorityRejectedAssignmentMismatch, "human RoleAssignment is incomplete")
	}
	assignmentCoversWork := !workWindow.From().Before(assignmentWindow.From()) &&
		!workWindow.Until().After(assignmentWindow.Until())
	if assignmentContext != content.JudgementContext() || !assignmentCoversWork {
		return reject(
			AuthorityRejectedAssignmentMismatch,
			"human RoleAssignment context/window mismatch",
		)
	}
	return nil
}

func projectAuthorityResolutionBasis(
	basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis,
) (authorityResolutionBasisProjection, error) {
	source := basis.record.Source()
	speechAct, speechActOK := source.SpeechActRef()
	work, workOK := source.WorkRef()
	sourceDigest, sourceDigestOK := source.Digest()
	projectRoot, projectRootOK := source.ProjectRoot()
	sourcePolicy, sourcePolicyOK := source.ContextPolicy()
	sourcePolicyRef, sourcePolicyRefOK := sourcePolicy.Ref()
	sourcePolicyDigest, sourcePolicyDigestOK := sourcePolicy.Digest()
	assignment, assignmentOK := source.PerformedByRoleAssignment()
	assignmentRef, assignmentRefOK := assignment.Ref()
	assignmentDigest, assignmentDigestOK := assignment.Digest()
	holder, holderOK := assignment.HolderSystemRef()
	role, roleOK := assignment.RoleRef()
	present := speechActOK && workOK && sourceDigestOK && projectRootOK &&
		sourcePolicyOK && sourcePolicyRefOK && sourcePolicyDigestOK &&
		assignmentOK && assignmentRefOK && assignmentDigestOK && holderOK && roleOK
	if !present {
		return authorityResolutionBasisProjection{}, fmt.Errorf(
			"authority resolution basis source coordinates are incomplete",
		)
	}
	head, err := basis.request.Head()
	if err != nil {
		return authorityResolutionBasisProjection{}, err
	}
	predecessor, err := projectPredecessor(basis.request.Predecessor())
	if err != nil {
		return authorityResolutionBasisProjection{}, err
	}
	target := basis.request.Target()
	return authorityResolutionBasisProjection{
		Schema:                 authorityResolutionBasisSchema,
		EvaluatedAt:            formatTime(basis.evaluatedAt),
		ResolverPolicyRef:      basis.policy.Ref().String(),
		ResolverPolicyEdition:  basis.policy.Edition().String(),
		ResolverPolicyDigest:   basis.policy.Digest().String(),
		SourceContractDigest:   basis.record.SourceContract().Digest().String(),
		SourceAdapterDigest:    basis.policy.SourceAdapter().Digest().String(),
		ProjectBindingDigest:   basis.record.ProjectBinding().Digest().String(),
		SpeechActRef:           speechAct.String(),
		WorkRef:                work.String(),
		SpeechActSourceDigest:  sourceDigest.String(),
		SpeechActRecordRef:     basis.record.Ref().String(),
		SpeechActRecordDigest:  basis.record.Digest().String(),
		ContentDescriptionKind: string(basis.content.DescriptionRef().Kind()),
		ContentDescriptionRef:  basis.content.DescriptionRef().String(),
		ContentDigest:          basis.content.Digest().String(),
		PermissionRef:          basis.record.PermissionRecord().Ref().String(),
		PermissionRecordDigest: basis.record.PermissionRecord().Digest().String(),
		RequestRef:             basis.request.Ref().String(),
		RequestDigest:          requestDigest(basis.request),
		Project:                basis.request.Project().String(),
		ProjectRoot:            projectRoot.String(),
		Head:                   head.String(),
		Predecessor:            predecessor,
		Action:                 basis.content.Action().String(),
		JudgementContext:       basis.content.JudgementContext().String(),
		ContextPolicyRef:       sourcePolicyRef.String(),
		ContextPolicyDigest:    sourcePolicyDigest.String(),
		RoleAssignmentRef:      assignmentRef.String(),
		RoleAssignmentDigest:   assignmentDigest.String(),
		HolderSystemRef:        holder.String(),
		RoleRef:                role.String(),
		ValidityFrom:           formatTime(basis.content.ValidityWindow().From()),
		ValidityUntil:          formatTime(basis.content.ValidityWindow().Until()),
		Stage:                  target.Stage().String(),
		VerifiedComposite:      target.VerifiedComposite().String(),
		ExpectedGraphRevision:  basis.request.ExpectedGraphRevision().Value(),
		IdempotencyKey:         basis.request.IdempotencyKey().String(),
		CurrentnessBoundary:    "pure_basis_does_not_prove_registry_currentness;_trusted_effect_must_reread_exact_policy_before_use",
	}, nil
}

func DecodeProjectTypeEnvHeadSelectionAuthorityResolutionBasis(
	input ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput,
	canonical []byte,
	digest authority.Digest,
) (ProjectTypeEnvHeadSelectionAuthorityResolutionBasis, error) {
	if len(canonical) == 0 || len(canonical) > maximumAuthorityResolutionBasisBytes {
		return ProjectTypeEnvHeadSelectionAuthorityResolutionBasis{}, fmt.Errorf(
			"TypeEnv authority resolution basis has invalid canonical size",
		)
	}
	projection := authorityResolutionBasisProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityResolutionBasis{}, err
	}
	rebuilt, err := SealProjectTypeEnvHeadSelectionAuthorityResolutionBasis(input)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityResolutionBasis{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return ProjectTypeEnvHeadSelectionAuthorityResolutionBasis{}, fmt.Errorf(
			"TypeEnv authority resolution basis is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis) Ref() ProjectTypeEnvHeadSelectionAuthorityResolutionBasisRef {
	return basis.ref
}

func (basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis) Digest() authority.Digest {
	return basis.digest
}

func (basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis) Policy() ProjectTypeEnvHeadSelectionResolverPolicy {
	return basis.policy
}

func (basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis) Record() ProjectTypeEnvHeadSelectionSpeechActRecord {
	return basis.record
}

func (basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis) Content() ProjectTypeEnvHeadSelectionAuthorizationContent {
	return basis.content
}

func (basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis) Request() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest {
	return basis.request
}

func (basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis) Stage() projecttypeenvselection.ProjectTypeEnvStage {
	return basis.stage
}

func (basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis) EvaluatedAt() time.Time {
	return basis.evaluatedAt
}

func (basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis) CanonicalJSON() []byte {
	return append([]byte(nil), basis.canonicalJSON...)
}

func (basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis) Verify(
	input ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput,
) error {
	rebuilt, err := SealProjectTypeEnvHeadSelectionAuthorityResolutionBasis(input)
	if err != nil {
		return err
	}
	if rebuilt.ref != basis.ref || rebuilt.digest != basis.digest ||
		!bytes.Equal(rebuilt.canonicalJSON, basis.canonicalJSON) {
		return fmt.Errorf("TypeEnv authority resolution basis differs from exact input")
	}
	return nil
}

func reject(code AuthorityRejectionCode, detail string) AuthorityRejection {
	return AuthorityRejection{code: code, detail: detail}
}
