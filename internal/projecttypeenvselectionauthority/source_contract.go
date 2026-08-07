package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
)

const (
	sourceContractSchema = "haft.project-typeenv.head-selection-speech-act-source-contract/v2"
	sourceContractDomain = "haft.project-typeenv.head-selection-speech-act-source-contract/v2"

	typeEnvSpeechActTypeRefValue    = "speech-act-type:authorize-project-typeenv-head-selection"
	typeEnvSpeechActStatePlaneValue = "state-plane:communicative-governance"
	typeEnvSpeechActDeltaValue      = "delta:reviewed-head-selection-content-placed-and-recognized"
	typeEnvSpeechActRoleRefValue    = "role:project-principal-authorizer"
)

// ProjectTypeEnvHeadSelectionSpeechActSourceContract is the reusable
// domain-semantic expectation for a human communicative Work occurrence. It
// deliberately says nothing about terminal capture, a concrete
// MethodDescription, or the system executing that method. Those are source
// adapter choices admitted by a resolver-policy edition, not TypeEnv meaning.
//
// Exact equality with the communicative StatePlane, delta, role, and the
// content-addressed Permission instituted by this SpeechAct makes the future
// head CAS inexpressible as this human act. No forbidden-word heuristic is
// involved.
type ProjectTypeEnvHeadSelectionSpeechActSourceContract struct {
	context       authority.BoundedContextRef
	actType       authority.SpeechActTypeRef
	statePlane    authority.StatePlaneRef
	delta         authority.DeltaPredicateRef
	role          authority.RoleRef
	digest        authority.Digest
	canonicalJSON []byte
}

type sourceContractProjection struct {
	Schema                 string `json:"schema"`
	JudgementContext       string `json:"judgement_context_ref"`
	ActTypeRef             string `json:"act_type_ref"`
	StatePlaneRef          string `json:"state_plane_ref"`
	DeltaPredicateRef      string `json:"delta_predicate_ref"`
	RoleRef                string `json:"role_ref"`
	InstitutedKind         string `json:"instituted_kind"`
	Modality               string `json:"modality"`
	PermissionIdentityRule string `json:"permission_identity_rule"`
}

func NewProjectTypeEnvHeadSelectionSpeechActSourceContract(
	context authority.BoundedContextRef,
) (ProjectTypeEnvHeadSelectionSpeechActSourceContract, error) {
	canonicalContext, err := authority.NewBoundedContextRef(context.String())
	if err != nil || canonicalContext != context {
		return ProjectTypeEnvHeadSelectionSpeechActSourceContract{}, fmt.Errorf(
			"source contract context is invalid",
		)
	}
	actType, err := authority.NewSpeechActTypeRef(typeEnvSpeechActTypeRefValue)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActSourceContract{}, err
	}
	statePlane, err := authority.NewStatePlaneRef(typeEnvSpeechActStatePlaneValue)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActSourceContract{}, err
	}
	delta, err := authority.NewDeltaPredicateRef(typeEnvSpeechActDeltaValue)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActSourceContract{}, err
	}
	role, err := authority.NewRoleRef(typeEnvSpeechActRoleRefValue)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActSourceContract{}, err
	}
	contract := ProjectTypeEnvHeadSelectionSpeechActSourceContract{
		context:    context,
		actType:    actType,
		statePlane: statePlane,
		delta:      delta,
		role:       role,
	}
	canonical, err := json.Marshal(projectSourceContract(contract))
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActSourceContract{}, err
	}
	digest, err := digestCanonical(sourceContractDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActSourceContract{}, err
	}
	contract.digest = digest
	contract.canonicalJSON = canonical
	return contract, nil
}

func projectSourceContract(
	contract ProjectTypeEnvHeadSelectionSpeechActSourceContract,
) sourceContractProjection {
	return sourceContractProjection{
		Schema:                 sourceContractSchema,
		JudgementContext:       contract.context.String(),
		ActTypeRef:             contract.actType.String(),
		StatePlaneRef:          contract.statePlane.String(),
		DeltaPredicateRef:      contract.delta.String(),
		RoleRef:                contract.role.String(),
		InstitutedKind:         "U.Commitment",
		Modality:               "MAY",
		PermissionIdentityRule: "haft.project-typeenv.head-selection-permission-identity/v2",
	}
}

func (contract ProjectTypeEnvHeadSelectionSpeechActSourceContract) VerifySource(
	source authority.VerifiedSpeechActSourceV2,
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) error {
	request := content.Request()
	if err := request.Verify(); err != nil {
		return err
	}
	speechAct, speechActOK := source.SpeechActRef()
	if !speechActOK {
		return fmt.Errorf("TypeEnv SpeechActRef is unavailable")
	}
	permissionRef, err := DeriveProjectTypeEnvHeadSelectionPermissionRef(content, speechAct)
	if err != nil {
		return err
	}
	expectedPermission, err := authority.NewInstitutedObjectRef(permissionRef.String())
	if err != nil {
		return err
	}
	expectedAction, err := content.Action().AuthorityActionKind()
	if err != nil {
		return err
	}
	contextPolicy, contextPolicyOK := source.ContextPolicy()
	institutedKind, institutedKindOK := contextPolicy.InstitutedObjectKind()
	modality, modalityOK := contextPolicy.InstitutionalModality()
	scopedAction, scopedActionOK := contextPolicy.ScopedAction()
	actTypes, actTypesOK := source.ActTypeRefs()
	statePlane, statePlaneOK := source.StatePlaneRef()
	delta, deltaOK := source.DeltaPredicateRef()
	assignment, assignmentOK := source.PerformedByRoleAssignment()
	role, roleOK := assignment.RoleRef()
	instituted, institutedOK := source.InstitutedObjectRef()
	context, contextOK := source.BoundedContext()
	workWindow, workWindowOK := source.WorkWindow()
	present := contextPolicyOK && institutedKindOK && modalityOK && scopedActionOK &&
		actTypesOK && statePlaneOK && deltaOK && assignmentOK && roleOK &&
		institutedOK && contextOK && workWindowOK
	if !present {
		return fmt.Errorf("TypeEnv SpeechAct semantic source coordinates are incomplete")
	}
	matches := len(actTypes) == 1 && actTypes[0] == contract.actType &&
		institutedKind.String() == "U.Commitment" &&
		modality.String() == "MAY" &&
		scopedAction == expectedAction &&
		statePlane == contract.statePlane &&
		delta == contract.delta &&
		role == contract.role &&
		instituted == expectedPermission &&
		context == contract.context
	if !matches {
		return fmt.Errorf("generic v2 source does not satisfy TypeEnv communicative semantics")
	}
	validity := content.ValidityWindow()
	workInsideValidity := !workWindow.From().Before(validity.From()) &&
		!workWindow.Until().After(validity.Until())
	if !workInsideValidity {
		return fmt.Errorf("TypeEnv SpeechAct Work is outside reviewed content validity")
	}
	return nil
}

func DecodeProjectTypeEnvHeadSelectionSpeechActSourceContract(
	context authority.BoundedContextRef,
	canonical []byte,
	digest authority.Digest,
) (ProjectTypeEnvHeadSelectionSpeechActSourceContract, error) {
	if len(canonical) == 0 || len(canonical) > 64*1024 {
		return ProjectTypeEnvHeadSelectionSpeechActSourceContract{}, fmt.Errorf(
			"TypeEnv SpeechAct source contract has invalid canonical size",
		)
	}
	projection := sourceContractProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActSourceContract{}, err
	}
	rebuilt, err := NewProjectTypeEnvHeadSelectionSpeechActSourceContract(context)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActSourceContract{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return ProjectTypeEnvHeadSelectionSpeechActSourceContract{}, fmt.Errorf(
			"TypeEnv SpeechAct source contract is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (contract ProjectTypeEnvHeadSelectionSpeechActSourceContract) Digest() authority.Digest {
	return contract.digest
}

func (contract ProjectTypeEnvHeadSelectionSpeechActSourceContract) Context() authority.BoundedContextRef {
	return contract.context
}

func (contract ProjectTypeEnvHeadSelectionSpeechActSourceContract) ActType() authority.SpeechActTypeRef {
	return contract.actType
}

func (contract ProjectTypeEnvHeadSelectionSpeechActSourceContract) StatePlane() authority.StatePlaneRef {
	return contract.statePlane
}

func (contract ProjectTypeEnvHeadSelectionSpeechActSourceContract) DeltaPredicate() authority.DeltaPredicateRef {
	return contract.delta
}

func (contract ProjectTypeEnvHeadSelectionSpeechActSourceContract) Role() authority.RoleRef {
	return contract.role
}

func (contract ProjectTypeEnvHeadSelectionSpeechActSourceContract) CanonicalJSON() []byte {
	return append([]byte(nil), contract.canonicalJSON...)
}

func (contract ProjectTypeEnvHeadSelectionSpeechActSourceContract) ExactAgainst(
	context authority.BoundedContextRef,
) error {
	rebuilt, err := NewProjectTypeEnvHeadSelectionSpeechActSourceContract(context)
	if err != nil {
		return err
	}
	if rebuilt.digest != contract.digest || !bytes.Equal(rebuilt.canonicalJSON, contract.canonicalJSON) {
		return fmt.Errorf("TypeEnv SpeechAct source contract differs from exact context")
	}
	return nil
}
