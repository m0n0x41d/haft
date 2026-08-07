package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
)

const (
	permissionRecordSchema        = "haft.project-typeenv.head-selection-permission-record/v3"
	permissionRecordDomain        = "haft.project-typeenv.head-selection-permission-record/v3"
	permissionIdentityDomain      = "haft.project-typeenv.head-selection-permission-identity/v2"
	permissionScopeDomain         = "haft.project-typeenv.head-selection-permission-scope/v1"
	permissionSubjectSchema       = "haft.project-typeenv.head-selection-permission-subject-role-assignment/v1"
	permissionSubjectDomain       = "haft.project-typeenv.head-selection-permission-subject-role-assignment/v1"
	permissionRefPrefix           = "project-typeenv-head-selection-permission:"
	permissionClaimScopePrefix    = "claim-scope:project-typeenv-head-selection:"
	permissionSubjectRefPrefix    = "role-assignment:haft-software-system:project-governance-substrate:"
	maximumPermissionSubjectBytes = 64 * 1024
	maximumPermissionRecordBytes  = 256 * 1024
)

// ProjectTypeEnvHeadSelectionPermissionRef identifies one instituted
// U.Commitment(MAY). The record digest separately authenticates its complete
// A.2.8 projection.
type ProjectTypeEnvHeadSelectionPermissionRef struct {
	digest authority.Digest
}

func ParseProjectTypeEnvHeadSelectionPermissionRef(
	raw string,
) (ProjectTypeEnvHeadSelectionPermissionRef, error) {
	digest, err := parseDigestRef(
		"head-selection Permission",
		permissionRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRef{}, err
	}
	return ProjectTypeEnvHeadSelectionPermissionRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionPermissionRef) String() string {
	return permissionRefPrefix + ref.digest.String()
}

func (ref ProjectTypeEnvHeadSelectionPermissionRef) Digest() authority.Digest {
	return ref.digest
}

// ProjectTypeEnvHeadSelectionPermissionModality is intentionally a one-value
// type: this effect-specific commitment can express Permission only.
type ProjectTypeEnvHeadSelectionPermissionModality uint8

const ProjectTypeEnvHeadSelectionPermissionMay ProjectTypeEnvHeadSelectionPermissionModality = 1

func (modality ProjectTypeEnvHeadSelectionPermissionModality) String() string {
	if modality == ProjectTypeEnvHeadSelectionPermissionMay {
		return "MAY"
	}
	return ""
}

// ProjectTypeEnvHeadSelectionPermissionSubject is the exact RoleAssignment
// that may enact the later head-selection CAS Work. It belongs to
// HaftSoftwareSystem and is deliberately distinct from the human
// project-principal-authorizer RoleAssignment that performed the instituting
// SpeechAct.
type ProjectTypeEnvHeadSelectionPermissionSubject struct {
	ref                        authority.RoleAssignmentRef
	digest                     authority.Digest
	project                    projectidentity.ProjectID
	holderSystem               authority.SystemRef
	role                       authority.RoleRef
	boundedContext             authority.BoundedContextRef
	assignmentWindow           authority.TimeWindow
	assignmentPolicy           ProjectTypeEnvHeadSelectionExecutionRolePolicy
	assignmentSupport          executionRoleAssignmentSupport
	authorizationDescription   authority.DescriptionRef
	authorizationContentDigest authority.Digest
	canonicalJSON              []byte
}

type permissionSubjectProjection struct {
	Schema                        string `json:"schema"`
	Project                       string `json:"project_id"`
	HolderSystemRef               string `json:"holder_system_ref"`
	HolderKind                    string `json:"holder_kind"`
	RoleRef                       string `json:"role_ref"`
	BoundedContextRef             string `json:"bounded_context_ref"`
	AssignmentFrom                string `json:"assignment_from"`
	AssignmentUntil               string `json:"assignment_until"`
	AssignmentPolicyRef           string `json:"assignment_policy_ref"`
	AssignmentPolicyDigest        string `json:"assignment_policy_digest"`
	AssignmentPolicyEditionRef    string `json:"assignment_policy_edition_ref"`
	AssignmentPolicySelection     string `json:"assignment_policy_selection"`
	SystemAdmissionRef            string `json:"system_admission_ref"`
	SystemAdmissionDigest         string `json:"system_admission_digest"`
	RoleAdmissionRef              string `json:"role_admission_ref"`
	RoleAdmissionDigest           string `json:"role_admission_digest"`
	AssignmentJustificationRef    string `json:"assignment_justification_ref"`
	AssignmentJustificationDigest string `json:"assignment_justification_digest"`
	AssignmentProvenanceRef       string `json:"assignment_provenance_ref"`
	AssignmentProvenanceDigest    string `json:"assignment_provenance_digest"`
	AuthorizationDescriptionKind  string `json:"authorization_description_kind"`
	AuthorizationDescriptionRef   string `json:"authorization_description_ref"`
	AuthorizationContentDigest    string `json:"authorization_content_digest"`
}

// SealProjectTypeEnvHeadSelectionPermissionSubject derives the one exact
// service execution RoleAssignment from reviewed content. The effect service
// uses the same assignment for later CAS Work in both authority modes; the
// strict Permission additionally names it as its accountable subject.
func SealProjectTypeEnvHeadSelectionPermissionSubject(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) (ProjectTypeEnvHeadSelectionPermissionSubject, error) {
	policy, err := CurrentProjectTypeEnvHeadSelectionExecutionRolePolicy()
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	return sealProjectTypeEnvHeadSelectionPermissionSubjectWithPolicy(
		content,
		policy,
		true,
	)
}

func sealProjectTypeEnvHeadSelectionPermissionSubjectWithPolicy(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	policy ProjectTypeEnvHeadSelectionExecutionRolePolicy,
	requireCurrentForNewWrite bool,
) (ProjectTypeEnvHeadSelectionPermissionSubject, error) {
	var support executionRoleAssignmentSupport
	var err error
	if requireCurrentForNewWrite {
		support, err = sealExecutionRoleAssignmentSupport(policy, content)
	} else {
		coordinates, coordinateErr := executionRoleAssignmentCoordinatesFromContent(content)
		if coordinateErr != nil {
			return ProjectTypeEnvHeadSelectionPermissionSubject{}, coordinateErr
		}
		support, err = sealExecutionRoleAssignmentSupportForCoordinates(
			policy,
			coordinates,
		)
	}
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	holderSystem := policy.HolderSystem()
	role := policy.Role()
	subject := ProjectTypeEnvHeadSelectionPermissionSubject{
		project:                    content.Project(),
		holderSystem:               holderSystem,
		role:                       role,
		boundedContext:             content.JudgementContext(),
		assignmentWindow:           content.ValidityWindow(),
		assignmentPolicy:           policy,
		assignmentSupport:          support,
		authorizationDescription:   content.DescriptionRef(),
		authorizationContentDigest: content.Digest(),
	}
	window := subject.assignmentWindow
	projection := permissionSubjectProjection{
		Schema:                        permissionSubjectSchema,
		Project:                       subject.project.String(),
		HolderSystemRef:               holderSystem.String(),
		HolderKind:                    "U.System",
		RoleRef:                       role.String(),
		BoundedContextRef:             subject.boundedContext.String(),
		AssignmentFrom:                formatTime(window.From()),
		AssignmentUntil:               formatTime(window.Until()),
		AssignmentPolicyRef:           policy.Ref().String(),
		AssignmentPolicyDigest:        policy.Digest().String(),
		AssignmentPolicyEditionRef:    policy.Edition().String(),
		AssignmentPolicySelection:     "current_for_new_write_at_seal",
		SystemAdmissionRef:            support.systemAdmissionRef.String(),
		SystemAdmissionDigest:         support.systemAdmissionDigest.String(),
		RoleAdmissionRef:              support.roleAdmissionRef.String(),
		RoleAdmissionDigest:           support.roleAdmissionDigest.String(),
		AssignmentJustificationRef:    support.justificationRef.String(),
		AssignmentJustificationDigest: support.justificationDigest.String(),
		AssignmentProvenanceRef:       support.provenanceRef.String(),
		AssignmentProvenanceDigest:    support.provenanceDigest.String(),
		AuthorizationDescriptionKind:  string(content.DescriptionRef().Kind()),
		AuthorizationDescriptionRef:   content.DescriptionRef().String(),
		AuthorizationContentDigest:    content.Digest().String(),
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	digest, err := digestCanonical(permissionSubjectDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	ref, err := authority.NewRoleAssignmentRef(
		permissionSubjectRefPrefix + digest.String(),
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	subject.ref = ref
	subject.digest = digest
	subject.canonicalJSON = canonical
	return subject, nil
}

// DecodeProjectTypeEnvHeadSelectionPermissionSubject recovers the exact
// canonical service RoleAssignment without needing authorization content.
// Reliance boundaries must still call Verify(content) against the reread
// content before using it.
func DecodeProjectTypeEnvHeadSelectionPermissionSubject(
	canonical []byte,
) (ProjectTypeEnvHeadSelectionPermissionSubject, error) {
	if len(canonical) == 0 || len(canonical) > maximumPermissionSubjectBytes {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, fmt.Errorf(
			"Permission subject has invalid canonical size",
		)
	}
	projection := permissionSubjectProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	policy, err := projectTypeEnvHeadSelectionExecutionRolePolicyForEdition(
		projection.AssignmentPolicyEditionRef,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	if projection.Schema != permissionSubjectSchema ||
		projection.HolderSystemRef != policy.HolderSystem().String() ||
		projection.HolderKind != "U.System" ||
		projection.RoleRef != policy.Role().String() ||
		projection.AssignmentPolicyRef != policy.Ref().String() ||
		projection.AssignmentPolicyDigest != policy.Digest().String() ||
		projection.AssignmentPolicyEditionRef != policy.Edition().String() ||
		projection.AssignmentPolicySelection != "current_for_new_write_at_seal" {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, fmt.Errorf(
			"Permission subject violates the closed service assignment",
		)
	}
	project, err := projectidentity.ParseProjectID(projection.Project)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	holderSystem, err := authority.NewSystemRef(projection.HolderSystemRef)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	role, err := authority.NewRoleRef(projection.RoleRef)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	context, err := authority.NewBoundedContextRef(projection.BoundedContextRef)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	assignmentFrom, err := parseTime(projection.AssignmentFrom)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	assignmentUntil, err := parseTime(projection.AssignmentUntil)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	assignmentWindow, err := authority.NewTimeWindow(
		assignmentFrom,
		assignmentUntil,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	descriptionKind := authority.DescriptionRefKind(
		projection.AuthorizationDescriptionKind,
	)
	description, err := parseDescriptionRef(
		descriptionKind,
		projection.AuthorizationDescriptionRef,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	contentDigest, err := authority.NewDigest(
		projection.AuthorizationContentDigest,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	support, err := rebuildExecutionRoleAssignmentSupport(
		projection.AssignmentPolicyEditionRef,
		project,
		context,
		assignmentWindow,
		description,
		contentDigest,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	if projection.SystemAdmissionRef != support.systemAdmissionRef.String() ||
		projection.SystemAdmissionDigest != support.systemAdmissionDigest.String() ||
		projection.RoleAdmissionRef != support.roleAdmissionRef.String() ||
		projection.RoleAdmissionDigest != support.roleAdmissionDigest.String() ||
		projection.AssignmentJustificationRef != support.justificationRef.String() ||
		projection.AssignmentJustificationDigest != support.justificationDigest.String() ||
		projection.AssignmentProvenanceRef != support.provenanceRef.String() ||
		projection.AssignmentProvenanceDigest != support.provenanceDigest.String() {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, fmt.Errorf(
			"Permission subject support refs or digests differ from exact registered policy material",
		)
	}
	normalized := permissionSubjectProjection{
		Schema:                        permissionSubjectSchema,
		Project:                       project.String(),
		HolderSystemRef:               holderSystem.String(),
		HolderKind:                    "U.System",
		RoleRef:                       role.String(),
		BoundedContextRef:             context.String(),
		AssignmentFrom:                formatTime(assignmentWindow.From()),
		AssignmentUntil:               formatTime(assignmentWindow.Until()),
		AssignmentPolicyRef:           policy.Ref().String(),
		AssignmentPolicyDigest:        policy.Digest().String(),
		AssignmentPolicyEditionRef:    policy.Edition().String(),
		AssignmentPolicySelection:     "current_for_new_write_at_seal",
		SystemAdmissionRef:            support.systemAdmissionRef.String(),
		SystemAdmissionDigest:         support.systemAdmissionDigest.String(),
		RoleAdmissionRef:              support.roleAdmissionRef.String(),
		RoleAdmissionDigest:           support.roleAdmissionDigest.String(),
		AssignmentJustificationRef:    support.justificationRef.String(),
		AssignmentJustificationDigest: support.justificationDigest.String(),
		AssignmentProvenanceRef:       support.provenanceRef.String(),
		AssignmentProvenanceDigest:    support.provenanceDigest.String(),
		AuthorizationDescriptionKind:  string(description.Kind()),
		AuthorizationDescriptionRef:   description.String(),
		AuthorizationContentDigest:    contentDigest.String(),
	}
	exactCanonical, err := json.Marshal(normalized)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	if !bytes.Equal(exactCanonical, canonical) {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, fmt.Errorf(
			"Permission subject is not exact canonical material",
		)
	}
	digest, err := digestCanonical(permissionSubjectDomain, exactCanonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	ref, err := authority.NewRoleAssignmentRef(
		permissionSubjectRefPrefix + digest.String(),
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionSubject{}, err
	}
	return ProjectTypeEnvHeadSelectionPermissionSubject{
		ref:                        ref,
		digest:                     digest,
		project:                    project,
		holderSystem:               holderSystem,
		role:                       role,
		boundedContext:             context,
		assignmentWindow:           assignmentWindow,
		assignmentPolicy:           policy,
		assignmentSupport:          support,
		authorizationDescription:   description,
		authorizationContentDigest: contentDigest,
		canonicalJSON:              exactCanonical,
	}, nil
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) Verify(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) error {
	return verifyHistoricalPermissionSubjectContent(subject, content)
}

// VerifyCurrentForUse is the reliance-boundary check for a new effect. It
// intentionally reselects the current registered execution-role policy and
// its source-validity window. Historical inspection and replay use Verify.
func (subject ProjectTypeEnvHeadSelectionPermissionSubject) VerifyCurrentForUse(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) error {
	rebuilt, err := SealProjectTypeEnvHeadSelectionPermissionSubject(content)
	if err != nil {
		return err
	}
	if rebuilt.ref != subject.ref ||
		rebuilt.digest != subject.digest ||
		!bytes.Equal(rebuilt.canonicalJSON, subject.canonicalJSON) {
		return fmt.Errorf("Permission subject differs from exact service assignment")
	}
	return nil
}

func verifyHistoricalPermissionSubjectContent(
	subject ProjectTypeEnvHeadSelectionPermissionSubject,
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) error {
	if err := content.Verify(); err != nil {
		return err
	}
	decoded, err := DecodeProjectTypeEnvHeadSelectionPermissionSubject(
		subject.CanonicalJSON(),
	)
	if err != nil {
		return err
	}
	exactSubject := decoded.ref == subject.ref &&
		decoded.digest == subject.digest &&
		bytes.Equal(decoded.canonicalJSON, subject.canonicalJSON)
	if !exactSubject {
		return fmt.Errorf(
			"Permission subject is not exact registered historical material",
		)
	}
	window := content.ValidityWindow()
	exactContent := subject.project == content.Project() &&
		subject.boundedContext == content.JudgementContext() &&
		subject.assignmentWindow.From().Equal(window.From()) &&
		subject.assignmentWindow.Until().Equal(window.Until()) &&
		subject.authorizationDescription == content.DescriptionRef() &&
		subject.authorizationContentDigest == content.Digest()
	if !exactContent {
		return fmt.Errorf(
			"Permission subject differs from the reviewed historical content",
		)
	}
	return nil
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) Ref() authority.RoleAssignmentRef {
	return subject.ref
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) Digest() authority.Digest {
	return subject.digest
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) Project() projectidentity.ProjectID {
	return subject.project
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) HolderSystemRef() authority.SystemRef {
	return subject.holderSystem
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) RoleRef() authority.RoleRef {
	return subject.role
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) BoundedContext() authority.BoundedContextRef {
	return subject.boundedContext
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) AssignmentWindow() authority.TimeWindow {
	return subject.assignmentWindow
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) AssignmentPolicy() ProjectTypeEnvHeadSelectionExecutionRolePolicy {
	return subject.assignmentPolicy
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) SystemAdmissionRef() ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef {
	return subject.assignmentSupport.systemAdmissionRef
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) SystemAdmissionDigest() authority.Digest {
	return subject.assignmentSupport.systemAdmissionDigest
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) SystemAdmissionCanonicalJSON() []byte {
	return append([]byte(nil), subject.assignmentSupport.systemAdmissionCanonical...)
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) RoleAdmissionRef() ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef {
	return subject.assignmentSupport.roleAdmissionRef
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) RoleAdmissionDigest() authority.Digest {
	return subject.assignmentSupport.roleAdmissionDigest
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) RoleAdmissionCanonicalJSON() []byte {
	return append([]byte(nil), subject.assignmentSupport.roleAdmissionCanonical...)
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) AssignmentJustificationRef() ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef {
	return subject.assignmentSupport.justificationRef
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) AssignmentJustificationDigest() authority.Digest {
	return subject.assignmentSupport.justificationDigest
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) AssignmentJustificationCanonicalJSON() []byte {
	return append([]byte(nil), subject.assignmentSupport.justificationCanonical...)
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) AssignmentProvenanceRef() ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef {
	return subject.assignmentSupport.provenanceRef
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) AssignmentProvenanceDigest() authority.Digest {
	return subject.assignmentSupport.provenanceDigest
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) AssignmentProvenanceCanonicalJSON() []byte {
	return append([]byte(nil), subject.assignmentSupport.provenanceCanonical...)
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) AuthorizationDescriptionRef() authority.DescriptionRef {
	return subject.authorizationDescription
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) AuthorizationContentDigest() authority.Digest {
	return subject.authorizationContentDigest
}

func (subject ProjectTypeEnvHeadSelectionPermissionSubject) CanonicalJSON() []byte {
	return append([]byte(nil), subject.canonicalJSON...)
}

// ProjectTypeEnvHeadSelectionPermissionScope is the exact local U.ClaimScope
// for one request/content/action under one source context policy. It is not the
// Work judgement context itself; the bounded context is one explicit scope
// coordinate among several.
type ProjectTypeEnvHeadSelectionPermissionScope struct {
	ref                 authority.ClaimScopeRef
	digest              authority.Digest
	boundedContext      authority.BoundedContextRef
	contextPolicyRef    authority.ContextPolicyRef
	contextPolicyDigest authority.Digest
	canonicalJSON       []byte
}

type permissionScopeProjection struct {
	Schema              string `json:"schema"`
	Project             string `json:"project_id"`
	Action              string `json:"action"`
	RequestRef          string `json:"request_ref"`
	RequestDigest       string `json:"request_digest"`
	ContentRef          string `json:"content_ref"`
	ContentDigest       string `json:"content_digest"`
	BoundedContext      string `json:"bounded_context_ref"`
	ContextPolicyRef    string `json:"context_policy_ref"`
	ContextPolicyDigest string `json:"context_policy_digest"`
	ScopeBoundary       string `json:"scope_boundary"`
}

func sealProjectTypeEnvHeadSelectionPermissionScope(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	source authority.VerifiedSpeechActSourceV2,
) (ProjectTypeEnvHeadSelectionPermissionScope, error) {
	if err := content.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionPermissionScope{}, err
	}
	context, contextOK := source.BoundedContext()
	policy, policyOK := source.ContextPolicy()
	policyRef, policyRefOK := policy.Ref()
	policyDigest, policyDigestOK := policy.Digest()
	policyContext, policyContextOK := policy.BoundedContext()
	scopedAction, scopedActionOK := policy.ScopedAction()
	expectedAction, actionErr := content.Action().AuthorityActionKind()
	present := contextOK &&
		policyOK &&
		policyRefOK &&
		policyDigestOK &&
		policyContextOK &&
		scopedActionOK &&
		actionErr == nil
	if !present {
		return ProjectTypeEnvHeadSelectionPermissionScope{}, fmt.Errorf(
			"Permission scope source policy coordinates are incomplete",
		)
	}
	if context != content.JudgementContext() ||
		policyContext != content.JudgementContext() ||
		scopedAction != expectedAction {
		return ProjectTypeEnvHeadSelectionPermissionScope{}, fmt.Errorf(
			"Permission scope does not match reviewed content and source policy",
		)
	}
	projection := permissionScopeProjection{
		Schema:              "haft.project-typeenv.head-selection-permission-scope/v1",
		Project:             content.Project().String(),
		Action:              content.Action().String(),
		RequestRef:          content.Request().Ref().String(),
		RequestDigest:       requestDigest(content.Request()),
		ContentRef:          content.DescriptionRef().String(),
		ContentDigest:       content.Digest().String(),
		BoundedContext:      context.String(),
		ContextPolicyRef:    policyRef.String(),
		ContextPolicyDigest: policyDigest.String(),
		ScopeBoundary:       "exact_project_request_content_action_and_context_policy",
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionScope{}, err
	}
	digest, err := digestCanonical(permissionScopeDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionScope{}, err
	}
	ref, err := authority.NewClaimScopeRef(
		permissionClaimScopePrefix + digest.String(),
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionScope{}, err
	}
	return ProjectTypeEnvHeadSelectionPermissionScope{
		ref:                 ref,
		digest:              digest,
		boundedContext:      context,
		contextPolicyRef:    policyRef,
		contextPolicyDigest: policyDigest,
		canonicalJSON:       canonical,
	}, nil
}

func (scope ProjectTypeEnvHeadSelectionPermissionScope) Ref() authority.ClaimScopeRef {
	return scope.ref
}

func (scope ProjectTypeEnvHeadSelectionPermissionScope) Digest() authority.Digest {
	return scope.digest
}

func (scope ProjectTypeEnvHeadSelectionPermissionScope) BoundedContext() authority.BoundedContextRef {
	return scope.boundedContext
}

func (scope ProjectTypeEnvHeadSelectionPermissionScope) ContextPolicyRef() authority.ContextPolicyRef {
	return scope.contextPolicyRef
}

func (scope ProjectTypeEnvHeadSelectionPermissionScope) ContextPolicyDigest() authority.Digest {
	return scope.contextPolicyDigest
}

func (scope ProjectTypeEnvHeadSelectionPermissionScope) CanonicalJSON() []byte {
	return append([]byte(nil), scope.canonicalJSON...)
}

// ProjectTypeEnvHeadSelectionPermissionReferentKind keeps the commitment's
// payload references typed instead of relying on prose.
type ProjectTypeEnvHeadSelectionPermissionReferentKind uint8

const (
	ProjectTypeEnvHeadSelectionPermissionReferentAuthorizationContent ProjectTypeEnvHeadSelectionPermissionReferentKind = iota + 1
	ProjectTypeEnvHeadSelectionPermissionReferentSelectionRequest
)

func (kind ProjectTypeEnvHeadSelectionPermissionReferentKind) String() string {
	switch kind {
	case ProjectTypeEnvHeadSelectionPermissionReferentAuthorizationContent:
		return "authorization_content"
	case ProjectTypeEnvHeadSelectionPermissionReferentSelectionRequest:
		return "project_typeenv_head_selection_request"
	default:
		return ""
	}
}

// ProjectTypeEnvHeadSelectionPermissionReferent is one exact member of the
// nonempty referent set required by A.2.8.
type ProjectTypeEnvHeadSelectionPermissionReferent struct {
	kind   ProjectTypeEnvHeadSelectionPermissionReferentKind
	ref    string
	digest authority.Digest
}

func (referent ProjectTypeEnvHeadSelectionPermissionReferent) Kind() ProjectTypeEnvHeadSelectionPermissionReferentKind {
	return referent.kind
}

func (referent ProjectTypeEnvHeadSelectionPermissionReferent) Ref() string {
	return referent.ref
}

func (referent ProjectTypeEnvHeadSelectionPermissionReferent) Digest() authority.Digest {
	return referent.digest
}

type permissionReferentProjection struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

func projectTypeEnvHeadSelectionPermissionReferents(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) ([]ProjectTypeEnvHeadSelectionPermissionReferent, error) {
	requestDigestValue, err := authority.NewDigest(requestDigest(content.Request()))
	if err != nil {
		return nil, err
	}
	referents := []ProjectTypeEnvHeadSelectionPermissionReferent{
		{
			kind:   ProjectTypeEnvHeadSelectionPermissionReferentAuthorizationContent,
			ref:    content.DescriptionRef().String(),
			digest: content.Digest(),
		},
		{
			kind:   ProjectTypeEnvHeadSelectionPermissionReferentSelectionRequest,
			ref:    content.Request().Ref().String(),
			digest: requestDigestValue,
		},
	}
	sort.Slice(referents, func(left int, right int) bool {
		leftKind := referents[left].kind.String()
		rightKind := referents[right].kind.String()
		if leftKind != rightKind {
			return leftKind < rightKind
		}
		return referents[left].ref < referents[right].ref
	})
	if len(referents) == 0 {
		return nil, fmt.Errorf("Permission referents must be nonempty")
	}
	for _, referent := range referents {
		if referent.kind.String() == "" ||
			referent.ref == "" ||
			referent.digest.String() == "" {
			return nil, fmt.Errorf("Permission referent is incomplete")
		}
	}
	return referents, nil
}

func projectPermissionReferents(
	referents []ProjectTypeEnvHeadSelectionPermissionReferent,
) []permissionReferentProjection {
	result := make([]permissionReferentProjection, 0, len(referents))
	for _, referent := range referents {
		result = append(result, permissionReferentProjection{
			Kind:   referent.Kind().String(),
			Ref:    referent.Ref(),
			Digest: referent.Digest().String(),
		})
	}
	return result
}

// ProjectTypeEnvHeadSelectionPermissionRecord is the complete description of
// one exact U.Commitment(MAY) instituted by verified communicative Work.
// Object, SpeechAct, Work, description, carrier, and commitment remain
// distinct identities.
type ProjectTypeEnvHeadSelectionPermissionRecord struct {
	ref           ProjectTypeEnvHeadSelectionPermissionRef
	digest        authority.Digest
	subject       ProjectTypeEnvHeadSelectionPermissionSubject
	modality      ProjectTypeEnvHeadSelectionPermissionModality
	scope         ProjectTypeEnvHeadSelectionPermissionScope
	referents     []ProjectTypeEnvHeadSelectionPermissionReferent
	source        authority.VerifiedSpeechActSourceV2
	content       ProjectTypeEnvHeadSelectionAuthorizationContent
	effectiveFrom time.Time
	validityUntil time.Time
	canonicalJSON []byte
}

type permissionIdentityProjection struct {
	Schema                 string `json:"schema"`
	SpeechActRef           string `json:"speech_act_ref"`
	SubjectRoleAssignment  string `json:"subject_role_assignment_ref"`
	SubjectAssignmentDig   string `json:"subject_role_assignment_digest"`
	ContentDescriptionKind string `json:"content_description_ref_kind"`
	ContentDescriptionRef  string `json:"content_description_ref"`
	ContentDigest          string `json:"content_digest"`
	RequestRef             string `json:"request_ref"`
	RequestDigest          string `json:"request_digest"`
	Project                string `json:"project_id"`
	Action                 string `json:"action"`
	JudgementContext       string `json:"judgement_context_ref"`
}

type permissionRecordProjection struct {
	Schema                   string                         `json:"schema"`
	PermissionRef            string                         `json:"permission_ref"`
	InstitutedKind           string                         `json:"instituted_kind"`
	SubjectRoleAssignmentRef string                         `json:"subject_role_assignment_ref"`
	SubjectRoleAssignmentDig string                         `json:"subject_role_assignment_digest"`
	SubjectRoleAssignment    json.RawMessage                `json:"subject_role_assignment"`
	Modality                 string                         `json:"modality"`
	ClaimScopeRef            string                         `json:"claim_scope_ref"`
	ClaimScopeDigest         string                         `json:"claim_scope_digest"`
	ClaimScope               json.RawMessage                `json:"claim_scope"`
	Referents                []permissionReferentProjection `json:"referents"`
	SpeechActRef             string                         `json:"speech_act_ref"`
	SpeechActSourceDigest    string                         `json:"speech_act_source_digest"`
	SpeechActWorkRef         string                         `json:"speech_act_work_ref"`
	SpeechActWorkFrom        string                         `json:"speech_act_work_from"`
	SpeechActWorkUntil       string                         `json:"speech_act_work_until"`
	ContentDescriptionKind   string                         `json:"content_description_ref_kind"`
	ContentDescriptionRef    string                         `json:"content_description_ref"`
	ContentDigest            string                         `json:"content_digest"`
	RequestRef               string                         `json:"request_ref"`
	RequestDigest            string                         `json:"request_digest"`
	Project                  string                         `json:"project_id"`
	Action                   string                         `json:"action"`
	JudgementContext         string                         `json:"judgement_context_ref"`
	ContextPolicyRef         string                         `json:"context_policy_ref"`
	ContextPolicyDigest      string                         `json:"context_policy_digest"`
	EffectiveFrom            string                         `json:"effective_from"`
	ValidityUntil            string                         `json:"validity_until"`
	EffectBoundary           string                         `json:"effect_boundary"`
}

// DeriveProjectTypeEnvHeadSelectionPermissionRef derives the intended
// commitment identity from reviewed content plus planned SpeechAct identity.
// It does not mint a PermissionRecord; sealing requires verified Work.
func DeriveProjectTypeEnvHeadSelectionPermissionRef(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	speechAct authority.SpeechActRef,
) (ProjectTypeEnvHeadSelectionPermissionRef, error) {
	subject, err := SealProjectTypeEnvHeadSelectionPermissionSubject(content)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRef{}, err
	}
	return deriveProjectTypeEnvHeadSelectionPermissionRefWithSubject(
		content,
		speechAct,
		subject,
	)
}

func deriveProjectTypeEnvHeadSelectionPermissionRefWithSubject(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	speechAct authority.SpeechActRef,
	subject ProjectTypeEnvHeadSelectionPermissionSubject,
) (ProjectTypeEnvHeadSelectionPermissionRef, error) {
	if err := content.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRef{}, err
	}
	canonicalSpeechAct, err := authority.NewSpeechActRef(speechAct.String())
	if err != nil || canonicalSpeechAct != speechAct {
		return ProjectTypeEnvHeadSelectionPermissionRef{}, fmt.Errorf(
			"TypeEnv head-selection Permission SpeechActRef is invalid",
		)
	}
	if _, err := content.Action().AuthorityActionKind(); err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRef{}, err
	}
	if err := verifyHistoricalPermissionSubjectContent(
		subject,
		content,
	); err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRef{}, err
	}
	projection := permissionIdentityProjection{
		Schema:                 "haft.project-typeenv.head-selection-permission-identity/v2",
		SpeechActRef:           speechAct.String(),
		SubjectRoleAssignment:  subject.Ref().String(),
		SubjectAssignmentDig:   subject.Digest().String(),
		ContentDescriptionKind: string(content.DescriptionRef().Kind()),
		ContentDescriptionRef:  content.DescriptionRef().String(),
		ContentDigest:          content.Digest().String(),
		RequestRef:             content.Request().Ref().String(),
		RequestDigest:          requestDigest(content.Request()),
		Project:                content.Project().String(),
		Action:                 content.Action().String(),
		JudgementContext:       content.JudgementContext().String(),
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRef{}, err
	}
	digest, err := digestCanonical(permissionIdentityDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRef{}, err
	}
	return ProjectTypeEnvHeadSelectionPermissionRef{digest: digest}, nil
}

func SealProjectTypeEnvHeadSelectionPermissionRecord(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	source authority.VerifiedSpeechActSourceV2,
) (ProjectTypeEnvHeadSelectionPermissionRecord, error) {
	subject, err := SealProjectTypeEnvHeadSelectionPermissionSubject(content)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	return sealProjectTypeEnvHeadSelectionPermissionRecordWithSubject(
		content,
		source,
		subject,
	)
}

func sealProjectTypeEnvHeadSelectionPermissionRecordWithSubject(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	source authority.VerifiedSpeechActSourceV2,
	subject ProjectTypeEnvHeadSelectionPermissionSubject,
) (ProjectTypeEnvHeadSelectionPermissionRecord, error) {
	if !source.Valid() {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, fmt.Errorf(
			"TypeEnv PermissionRecord requires verified SpeechAct Work",
		)
	}
	if err := content.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	speechAct, speechActOK := source.SpeechActRef()
	sourceDigest, sourceDigestOK := source.Digest()
	work, workOK := source.WorkRef()
	window, windowOK := source.WorkWindow()
	description, descriptionOK := source.DescriptionRef()
	descriptionDigest, descriptionDigestOK := source.DescriptionDigest()
	instituted, institutedOK := source.InstitutedObjectRef()
	sourceAssignment, sourceAssignmentOK := source.PerformedByRoleAssignment()
	sourceAssignmentRef, sourceAssignmentRefOK := sourceAssignment.Ref()
	sourceAssignmentDigest, sourceAssignmentDigestOK := sourceAssignment.Digest()
	present := speechActOK &&
		sourceDigestOK &&
		workOK &&
		windowOK &&
		descriptionOK &&
		descriptionDigestOK &&
		institutedOK &&
		sourceAssignmentOK &&
		sourceAssignmentRefOK &&
		sourceAssignmentDigestOK
	if !present {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, fmt.Errorf(
			"TypeEnv PermissionRecord source coordinates are incomplete",
		)
	}
	ref, err := deriveProjectTypeEnvHeadSelectionPermissionRefWithSubject(
		content,
		speechAct,
		subject,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	expectedInstituted, err := authority.NewInstitutedObjectRef(ref.String())
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	sourceMatches := description == content.DescriptionRef() &&
		descriptionDigest == content.Digest() &&
		instituted == expectedInstituted
	if !sourceMatches {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, fmt.Errorf(
			"verified SpeechAct Work does not institute the exact reviewed Permission",
		)
	}
	if subject.Ref() == sourceAssignmentRef ||
		subject.Digest() == sourceAssignmentDigest {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, fmt.Errorf(
			"Permission subject must remain distinct from instituting SpeechAct performer",
		)
	}
	scope, err := sealProjectTypeEnvHeadSelectionPermissionScope(
		content,
		source,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	referents, err := projectTypeEnvHeadSelectionPermissionReferents(content)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	validity := content.ValidityWindow()
	effectiveFrom := window.Until()
	if effectiveFrom.Before(validity.From()) {
		effectiveFrom = validity.From()
	}
	workInsideValidity := !window.From().Before(validity.From()) &&
		!window.Until().After(validity.Until())
	if !workInsideValidity || !effectiveFrom.Before(validity.Until()) {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, fmt.Errorf(
			"instituting SpeechAct Work leaves no valid Permission interval",
		)
	}
	record := ProjectTypeEnvHeadSelectionPermissionRecord{
		ref:           ref,
		subject:       subject,
		modality:      ProjectTypeEnvHeadSelectionPermissionMay,
		scope:         scope,
		referents:     referents,
		source:        source,
		content:       content,
		effectiveFrom: effectiveFrom,
		validityUntil: validity.Until(),
	}
	projection, err := projectPermissionRecord(
		record,
		speechAct,
		sourceDigest,
		work,
		window,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	if len(canonical) == 0 || len(canonical) > maximumPermissionRecordBytes {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, fmt.Errorf(
			"TypeEnv PermissionRecord has invalid canonical size",
		)
	}
	digest, err := digestCanonical(permissionRecordDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	record.digest = digest
	record.canonicalJSON = canonical
	return record, nil
}

func projectPermissionRecord(
	record ProjectTypeEnvHeadSelectionPermissionRecord,
	speechAct authority.SpeechActRef,
	sourceDigest authority.Digest,
	work authority.WorkRef,
	window authority.TimeWindow,
) (permissionRecordProjection, error) {
	policy, policyOK := record.source.ContextPolicy()
	policyRef, policyRefOK := policy.Ref()
	policyDigest, policyDigestOK := policy.Digest()
	if !policyOK || !policyRefOK || !policyDigestOK {
		return permissionRecordProjection{}, fmt.Errorf(
			"Permission context policy coordinates are unavailable",
		)
	}
	content := record.content
	return permissionRecordProjection{
		Schema:                   permissionRecordSchema,
		PermissionRef:            record.ref.String(),
		InstitutedKind:           "U.Commitment",
		SubjectRoleAssignmentRef: record.subject.Ref().String(),
		SubjectRoleAssignmentDig: record.subject.Digest().String(),
		SubjectRoleAssignment:    json.RawMessage(record.subject.CanonicalJSON()),
		Modality:                 record.modality.String(),
		ClaimScopeRef:            record.scope.Ref().String(),
		ClaimScopeDigest:         record.scope.Digest().String(),
		ClaimScope:               json.RawMessage(record.scope.CanonicalJSON()),
		Referents:                projectPermissionReferents(record.referents),
		SpeechActRef:             speechAct.String(),
		SpeechActSourceDigest:    sourceDigest.String(),
		SpeechActWorkRef:         work.String(),
		SpeechActWorkFrom:        formatTime(window.From()),
		SpeechActWorkUntil:       formatTime(window.Until()),
		ContentDescriptionKind:   string(content.DescriptionRef().Kind()),
		ContentDescriptionRef:    content.DescriptionRef().String(),
		ContentDigest:            content.Digest().String(),
		RequestRef:               content.Request().Ref().String(),
		RequestDigest:            requestDigest(content.Request()),
		Project:                  content.Project().String(),
		Action:                   content.Action().String(),
		JudgementContext:         content.JudgementContext().String(),
		ContextPolicyRef:         policyRef.String(),
		ContextPolicyDigest:      policyDigest.String(),
		EffectiveFrom:            formatTime(record.effectiveFrom),
		ValidityUntil:            formatTime(record.validityUntil),
		EffectBoundary:           "U.Commitment(MAY)_for_exact_typed_referents;_not_speech_act_work_description_carrier_or_head_mutation",
	}, nil
}

func DecodeProjectTypeEnvHeadSelectionPermissionRecord(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	source authority.VerifiedSpeechActSourceV2,
	canonical []byte,
	digest authority.Digest,
) (ProjectTypeEnvHeadSelectionPermissionRecord, error) {
	if len(canonical) == 0 || len(canonical) > maximumPermissionRecordBytes {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, fmt.Errorf(
			"TypeEnv PermissionRecord has invalid canonical size",
		)
	}
	projection := permissionRecordProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	subject, err := DecodeProjectTypeEnvHeadSelectionPermissionSubject(
		projection.SubjectRoleAssignment,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	rebuilt, err := sealProjectTypeEnvHeadSelectionPermissionRecordWithSubject(
		content,
		source,
		subject,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return ProjectTypeEnvHeadSelectionPermissionRecord{}, fmt.Errorf(
			"TypeEnv PermissionRecord is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) Verify(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	source authority.VerifiedSpeechActSourceV2,
) error {
	rebuilt, err := sealProjectTypeEnvHeadSelectionPermissionRecordWithSubject(
		content,
		source,
		record.subject,
	)
	if err != nil {
		return err
	}
	if rebuilt.ref != record.ref ||
		rebuilt.digest != record.digest ||
		!bytes.Equal(rebuilt.canonicalJSON, record.canonicalJSON) {
		return fmt.Errorf("TypeEnv PermissionRecord differs from exact source")
	}
	return nil
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) Ref() ProjectTypeEnvHeadSelectionPermissionRef {
	return record.ref
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) Digest() authority.Digest {
	return record.digest
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) SubjectRoleAssignmentRef() authority.RoleAssignmentRef {
	return record.subject.Ref()
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) SubjectRoleAssignmentDigest() authority.Digest {
	return record.subject.Digest()
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) Subject() ProjectTypeEnvHeadSelectionPermissionSubject {
	return record.subject
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) Modality() ProjectTypeEnvHeadSelectionPermissionModality {
	return record.modality
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) Scope() ProjectTypeEnvHeadSelectionPermissionScope {
	return record.scope
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) Referents() []ProjectTypeEnvHeadSelectionPermissionReferent {
	return append([]ProjectTypeEnvHeadSelectionPermissionReferent(nil), record.referents...)
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) EffectiveFrom() time.Time {
	return record.effectiveFrom
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) ValidityUntil() time.Time {
	return record.validityUntil
}

func (record ProjectTypeEnvHeadSelectionPermissionRecord) CanonicalJSON() []byte {
	return append([]byte(nil), record.canonicalJSON...)
}
