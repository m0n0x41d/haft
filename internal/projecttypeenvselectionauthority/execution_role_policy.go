package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
)

const (
	executionRolePolicySchema    = "haft.project-typeenv.head-selection-execution-role-policy/v1"
	executionRolePolicyDomain    = "haft.project-typeenv.head-selection-execution-role-policy/v1"
	executionRolePolicyRefPrefix = "project-typeenv-head-selection-execution-role-policy:"

	executionSystemAdmissionSchema    = "haft.project-typeenv.head-selection-execution-system-admission/v1"
	executionSystemAdmissionDomain    = "haft.project-typeenv.head-selection-execution-system-admission/v1"
	executionSystemAdmissionRefPrefix = "system-admission:project-typeenv-head-selection:"

	executionRoleAdmissionSchema    = "haft.project-typeenv.head-selection-execution-role-admission/v1"
	executionRoleAdmissionDomain    = "haft.project-typeenv.head-selection-execution-role-admission/v1"
	executionRoleAdmissionRefPrefix = "role-admission:project-typeenv-head-selection:"

	executionAssignmentJustificationSchema    = "haft.project-typeenv.head-selection-execution-assignment-justification/v1"
	executionAssignmentJustificationDomain    = "haft.project-typeenv.head-selection-execution-assignment-justification/v1"
	executionAssignmentJustificationRefPrefix = "role-assignment-justification:project-typeenv-head-selection:"

	executionAssignmentProvenanceSchema    = "haft.project-typeenv.head-selection-execution-assignment-provenance/v1"
	executionAssignmentProvenanceDomain    = "haft.project-typeenv.head-selection-execution-assignment-provenance/v1"
	executionAssignmentProvenanceRefPrefix = "role-assignment-provenance:project-typeenv-head-selection:"

	executionRolePolicyEditionV1 = "policy-edition:project-typeenv-head-selection-execution-role/v1"
	currentExecutionRolePolicy   = executionRolePolicyEditionV1
	executionRoleHolderSystem    = "system:haft-software-system"
	executionRole                = "role:project-governance-substrate"

	executionRoleSystemPattern     = "A.1"
	executionRolePattern           = "A.2"
	executionRoleAssignmentPattern = "A.2.1"

	executionRoleFPFRevision   = "44dd88188a07646ef23aca32627a3f670525853f"
	executionRoleFPFSpecDigest = "sha256:e23120498005176fadbddb0b6d495f5e3bd8e0fdf60d45064db548845984be2a"

	executionRoleSSSectionRef    = "SS.role.001"
	executionRoleSSSectionDigest = "sha256:f5cf317cd6b869781e2215e544c5f0ced0dce46e0a8f6ac5f92f3206e85aafe1"
	executionRoleTSSectionRef    = "TS.role.001"
	executionRoleTSSectionDigest = "sha256:23188f4fc365e23cdc624483f59526da69c2946f311c6ac9c09e4900808cba58"

	executionRoleSSCarrierRef    = "carrier:.haft/specs/software-system.md"
	executionRoleSSCarrierDigest = "sha256:2c72ec9740d70af8ed45ffe8647f43009d2b71414cdaa330aae425ed8e5eac4f"
	executionRoleTSCarrierRef    = "carrier:.haft/specs/target-system.md"
	executionRoleTSCarrierDigest = "sha256:08e2059114caf77524cf57d1ebb3bc3e5af708a8ca8284388e0069800c458db0"

	executionRoleSSBaselineCapturedAt = "2026-07-15T19:44:27.046111Z"
	executionRoleSSValidUntil         = "2026-10-31T23:59:59Z"
	executionRoleTSBaselineCapturedAt = "2026-07-15T19:44:26.946907Z"
	executionRoleTSValidUntil         = "2026-09-18T23:59:59Z"
	executionRoleSourceApprovedBy     = "ivanzakutnii"

	executionRoleContractPayloadV1 = "This policy admits HaftSoftwareSystem as holder of the ProjectGovernanceSubstrate role in one exact project BoundedContext and finite assignment window. Source-grounded FPF retrieval and typed-project-memory capabilities are neighboring role qualification conditions. The RoleAssignment alone establishes neither a capability, gate passage, Permission, nor performed Work; separately governed method, authority, and evidence relations decide whether a head-selection Work occurrence may happen."
	executionRoleContractDomainV1  = "haft.project-typeenv.head-selection-execution-role-contract/v1"

	executionAssignmentRuleRefV1 = "rule:project-typeenv-head-selection/execution-role-assignment/v1"
	executionAssignmentRuleV1    = "admit the exact HaftSoftwareSystem ProjectGovernanceSubstrate RoleAssignment only when its independently sealed A.1 system admission and A.2 Role admission support the A.2.1 assignment in one project BoundedContext, current policy edition, and finite authorization-content window"

	maximumExecutionRolePolicyBytes = 256 * 1024
)

type ProjectTypeEnvHeadSelectionExecutionRolePolicyRef struct {
	digest authority.Digest
}

func ParseProjectTypeEnvHeadSelectionExecutionRolePolicyRef(
	raw string,
) (ProjectTypeEnvHeadSelectionExecutionRolePolicyRef, error) {
	digest, err := parseDigestRef(
		"head-selection execution RolePolicy",
		executionRolePolicyRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicyRef{}, err
	}
	return ProjectTypeEnvHeadSelectionExecutionRolePolicyRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionExecutionRolePolicyRef) String() string {
	return executionRolePolicyRefPrefix + ref.digest.String()
}

func (ref ProjectTypeEnvHeadSelectionExecutionRolePolicyRef) Digest() authority.Digest {
	return ref.digest
}

type ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef struct {
	digest authority.Digest
}

func ParseProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef(
	raw string,
) (ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef, error) {
	digest, err := parseDigestRef(
		"head-selection execution system admission",
		executionSystemAdmissionRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef{}, err
	}
	return ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef) String() string {
	return executionSystemAdmissionRefPrefix + ref.digest.String()
}

func (ref ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef) Digest() authority.Digest {
	return ref.digest
}

type ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef struct {
	digest authority.Digest
}

func ParseProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef(
	raw string,
) (ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef, error) {
	digest, err := parseDigestRef(
		"head-selection execution role admission",
		executionRoleAdmissionRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef{}, err
	}
	return ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef) String() string {
	return executionRoleAdmissionRefPrefix + ref.digest.String()
}

func (ref ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef) Digest() authority.Digest {
	return ref.digest
}

type ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef struct {
	digest authority.Digest
}

func ParseProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef(
	raw string,
) (ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef, error) {
	digest, err := parseDigestRef(
		"head-selection execution assignment justification",
		executionAssignmentJustificationRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef{}, err
	}
	return ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef) String() string {
	return executionAssignmentJustificationRefPrefix + ref.digest.String()
}

func (ref ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef) Digest() authority.Digest {
	return ref.digest
}

type ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef struct {
	digest authority.Digest
}

func ParseProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef(
	raw string,
) (ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef, error) {
	digest, err := parseDigestRef(
		"head-selection execution assignment provenance",
		executionAssignmentProvenanceRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef{}, err
	}
	return ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef) String() string {
	return executionAssignmentProvenanceRefPrefix + ref.digest.String()
}

func (ref ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef) Digest() authority.Digest {
	return ref.digest
}

type executionRolePolicySourcePin struct {
	SectionRef         string `json:"section_ref"`
	SectionDigest      string `json:"section_digest"`
	CarrierRef         string `json:"carrier_ref"`
	CarrierDigest      string `json:"carrier_digest"`
	Lifecycle          string `json:"lifecycle"`
	ApprovedBy         string `json:"approved_by"`
	BaselineCapturedAt string `json:"baseline_captured_at"`
	ValidUntil         string `json:"valid_until"`
}

// ProjectTypeEnvHeadSelectionExecutionRolePolicy is a built-in, immutable
// policy edition. It embeds the accepted product-contract semantics and their
// provenance pins, so installed projects never need Haft's self-host specs at
// runtime. The closed edition registry selects one edition for new writes and
// retains decoders for historical editions.
type ProjectTypeEnvHeadSelectionExecutionRolePolicy struct {
	ref            ProjectTypeEnvHeadSelectionExecutionRolePolicyRef
	digest         authority.Digest
	edition        authority.PolicyVersion
	holderSystem   authority.SystemRef
	role           authority.RoleRef
	contractDigest authority.Digest
	sourceValidity authority.TimeWindow
	canonicalJSON  []byte
}

type executionRolePolicyProjection struct {
	Schema                     string                         `json:"schema"`
	PolicyEditionRef           string                         `json:"policy_edition_ref"`
	HolderSystemRef            string                         `json:"holder_system_ref"`
	AdmittedHolderKind         string                         `json:"admitted_holder_kind"`
	RoleRef                    string                         `json:"role_ref"`
	SystemGoverningPattern     string                         `json:"system_governing_pattern_ref"`
	RoleGoverningPattern       string                         `json:"role_governing_pattern_ref"`
	AssignmentGoverningPattern string                         `json:"assignment_governing_pattern_ref"`
	BundledFPFRevision         string                         `json:"bundled_fpf_revision"`
	BundledFPFSpecDigest       string                         `json:"bundled_fpf_spec_digest"`
	ProductContractPayload     string                         `json:"product_contract_payload"`
	ProductContractDigest      string                         `json:"product_contract_digest"`
	SourcePins                 []executionRolePolicySourcePin `json:"source_pins"`
	NewWriteSelection          string                         `json:"new_write_selection"`
	HistoricalDecode           string                         `json:"historical_decode"`
}

func CurrentProjectTypeEnvHeadSelectionExecutionRolePolicy() (
	ProjectTypeEnvHeadSelectionExecutionRolePolicy,
	error,
) {
	return projectTypeEnvHeadSelectionExecutionRolePolicyForEdition(
		currentExecutionRolePolicy,
	)
}

func projectTypeEnvHeadSelectionExecutionRolePolicyForEdition(
	edition string,
) (ProjectTypeEnvHeadSelectionExecutionRolePolicy, error) {
	switch edition {
	case executionRolePolicyEditionV1:
		return buildProjectTypeEnvHeadSelectionExecutionRolePolicyV1()
	default:
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, fmt.Errorf(
			"unsupported head-selection execution RolePolicy edition %q",
			edition,
		)
	}
}

func buildProjectTypeEnvHeadSelectionExecutionRolePolicyV1() (
	ProjectTypeEnvHeadSelectionExecutionRolePolicy,
	error,
) {
	edition, err := authority.NewPolicyVersion(executionRolePolicyEditionV1)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, err
	}
	holder, err := authority.NewSystemRef(executionRoleHolderSystem)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, err
	}
	role, err := authority.NewRoleRef(executionRole)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, err
	}
	contractDigest, err := digestCanonical(
		executionRoleContractDomainV1,
		[]byte(executionRoleContractPayloadV1),
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, err
	}
	projection := executionRolePolicyProjection{
		Schema:                     executionRolePolicySchema,
		PolicyEditionRef:           edition.String(),
		HolderSystemRef:            holder.String(),
		AdmittedHolderKind:         "U.System",
		RoleRef:                    role.String(),
		SystemGoverningPattern:     executionRoleSystemPattern,
		RoleGoverningPattern:       executionRolePattern,
		AssignmentGoverningPattern: executionRoleAssignmentPattern,
		BundledFPFRevision:         executionRoleFPFRevision,
		BundledFPFSpecDigest:       executionRoleFPFSpecDigest,
		ProductContractPayload:     executionRoleContractPayloadV1,
		ProductContractDigest:      contractDigest.String(),
		SourcePins: []executionRolePolicySourcePin{
			{
				SectionRef:         executionRoleSSSectionRef,
				SectionDigest:      executionRoleSSSectionDigest,
				CarrierRef:         executionRoleSSCarrierRef,
				CarrierDigest:      executionRoleSSCarrierDigest,
				Lifecycle:          "active_current_baseline",
				ApprovedBy:         executionRoleSourceApprovedBy,
				BaselineCapturedAt: executionRoleSSBaselineCapturedAt,
				ValidUntil:         executionRoleSSValidUntil,
			},
			{
				SectionRef:         executionRoleTSSectionRef,
				SectionDigest:      executionRoleTSSectionDigest,
				CarrierRef:         executionRoleTSCarrierRef,
				CarrierDigest:      executionRoleTSCarrierDigest,
				Lifecycle:          "active_current_baseline",
				ApprovedBy:         executionRoleSourceApprovedBy,
				BaselineCapturedAt: executionRoleTSBaselineCapturedAt,
				ValidUntil:         executionRoleTSValidUntil,
			},
		},
		NewWriteSelection: "closed_registry_current",
		HistoricalDecode:  "closed_registry_by_exact_edition",
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, err
	}
	digest, err := digestCanonical(executionRolePolicyDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, err
	}
	sourceValidity, err := executionRolePolicySourceValidity()
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, err
	}
	return ProjectTypeEnvHeadSelectionExecutionRolePolicy{
		ref: ProjectTypeEnvHeadSelectionExecutionRolePolicyRef{
			digest: digest,
		},
		digest:         digest,
		edition:        edition,
		holderSystem:   holder,
		role:           role,
		contractDigest: contractDigest,
		sourceValidity: sourceValidity,
		canonicalJSON:  canonical,
	}, nil
}

func DecodeProjectTypeEnvHeadSelectionExecutionRolePolicy(
	canonical []byte,
) (ProjectTypeEnvHeadSelectionExecutionRolePolicy, error) {
	if len(canonical) == 0 || len(canonical) > maximumExecutionRolePolicyBytes {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, fmt.Errorf(
			"head-selection execution RolePolicy has invalid canonical size",
		)
	}
	projection := executionRolePolicyProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, err
	}
	edition, err := projectTypeEnvHeadSelectionExecutionRolePolicyForEdition(
		projection.PolicyEditionRef,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, err
	}
	if !bytes.Equal(edition.canonicalJSON, canonical) {
		return ProjectTypeEnvHeadSelectionExecutionRolePolicy{}, fmt.Errorf(
			"head-selection execution RolePolicy is not exact registered edition material",
		)
	}
	return edition, nil
}

func (policy ProjectTypeEnvHeadSelectionExecutionRolePolicy) VerifyCurrentForNewWrite(
	window authority.TimeWindow,
) error {
	current, err := CurrentProjectTypeEnvHeadSelectionExecutionRolePolicy()
	if err != nil {
		return err
	}
	if current.ref != policy.ref ||
		current.digest != policy.digest ||
		!bytes.Equal(current.canonicalJSON, policy.canonicalJSON) {
		return fmt.Errorf(
			"head-selection execution RolePolicy is historical and cannot authorize a new write",
		)
	}
	if window.From().Before(policy.sourceValidity.From()) ||
		window.Until().After(policy.sourceValidity.Until()) {
		return fmt.Errorf(
			"policy_source_not_current_for_new_write: assignment window must fit current SS.role.001 and TS.role.001 baselines",
		)
	}
	return nil
}

func executionRolePolicySourceValidity() (authority.TimeWindow, error) {
	from, err := parseTime(executionRoleSSBaselineCapturedAt)
	if err != nil {
		return authority.TimeWindow{}, err
	}
	until, err := parseTime(executionRoleTSValidUntil)
	if err != nil {
		return authority.TimeWindow{}, err
	}
	return authority.NewTimeWindow(from, until)
}

func (policy ProjectTypeEnvHeadSelectionExecutionRolePolicy) Ref() ProjectTypeEnvHeadSelectionExecutionRolePolicyRef {
	return policy.ref
}

func (policy ProjectTypeEnvHeadSelectionExecutionRolePolicy) Digest() authority.Digest {
	return policy.digest
}

func (policy ProjectTypeEnvHeadSelectionExecutionRolePolicy) Edition() authority.PolicyVersion {
	return policy.edition
}

func (policy ProjectTypeEnvHeadSelectionExecutionRolePolicy) HolderSystem() authority.SystemRef {
	return policy.holderSystem
}

func (policy ProjectTypeEnvHeadSelectionExecutionRolePolicy) Role() authority.RoleRef {
	return policy.role
}

func (policy ProjectTypeEnvHeadSelectionExecutionRolePolicy) ContractDigest() authority.Digest {
	return policy.contractDigest
}

func (policy ProjectTypeEnvHeadSelectionExecutionRolePolicy) SourceValidity() authority.TimeWindow {
	return policy.sourceValidity
}

func (policy ProjectTypeEnvHeadSelectionExecutionRolePolicy) CanonicalJSON() []byte {
	return append([]byte(nil), policy.canonicalJSON...)
}

type executionSystemAdmissionProjection struct {
	Schema                string `json:"schema"`
	Project               string `json:"project_id"`
	SystemRef             string `json:"system_ref"`
	AdmittedSystemKind    string `json:"admitted_system_kind"`
	BoundedContextRef     string `json:"bounded_context_ref"`
	ValidFrom             string `json:"valid_from"`
	ValidUntil            string `json:"valid_until"`
	GoverningPatternRef   string `json:"governing_pattern_ref"`
	PolicyRef             string `json:"policy_ref"`
	PolicyDigest          string `json:"policy_digest"`
	PolicyEditionRef      string `json:"policy_edition_ref"`
	ProductContractDigest string `json:"product_contract_digest"`
}

type executionRoleAdmissionProjection struct {
	Schema                string `json:"schema"`
	Project               string `json:"project_id"`
	RoleRef               string `json:"role_ref"`
	BoundedContextRef     string `json:"bounded_context_ref"`
	ValidFrom             string `json:"valid_from"`
	ValidUntil            string `json:"valid_until"`
	GoverningPatternRef   string `json:"governing_pattern_ref"`
	PolicyRef             string `json:"policy_ref"`
	PolicyDigest          string `json:"policy_digest"`
	PolicyEditionRef      string `json:"policy_edition_ref"`
	ProductContractDigest string `json:"product_contract_digest"`
}

type executionAssignmentJustificationProjection struct {
	Schema                string `json:"schema"`
	Project               string `json:"project_id"`
	GoverningPatternRef   string `json:"governing_pattern_ref"`
	RuleRef               string `json:"rule_ref"`
	Rule                  string `json:"rule"`
	BoundedContextRef     string `json:"bounded_context_ref"`
	AssignmentFrom        string `json:"assignment_from"`
	AssignmentUntil       string `json:"assignment_until"`
	SystemAdmissionRef    string `json:"system_admission_ref"`
	SystemAdmissionDigest string `json:"system_admission_digest"`
	RoleAdmissionRef      string `json:"role_admission_ref"`
	RoleAdmissionDigest   string `json:"role_admission_digest"`
	PolicyRef             string `json:"policy_ref"`
	PolicyDigest          string `json:"policy_digest"`
	PolicyEditionRef      string `json:"policy_edition_ref"`
}

type executionAssignmentProvenanceProjection struct {
	Schema                       string                         `json:"schema"`
	Project                      string                         `json:"project_id"`
	JustificationRef             string                         `json:"justification_ref"`
	JustificationDigest          string                         `json:"justification_digest"`
	AuthorizationDescriptionKind string                         `json:"authorization_description_kind"`
	AuthorizationDescriptionRef  string                         `json:"authorization_description_ref"`
	AuthorizationContentDigest   string                         `json:"authorization_content_digest"`
	PolicyRef                    string                         `json:"policy_ref"`
	PolicyDigest                 string                         `json:"policy_digest"`
	PolicyEditionRef             string                         `json:"policy_edition_ref"`
	ProductContractDigest        string                         `json:"product_contract_digest"`
	SourcePins                   []executionRolePolicySourcePin `json:"source_pins"`
	Derivation                   string                         `json:"derivation"`
}

type executionRoleAssignmentSupport struct {
	systemAdmissionRef       ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef
	systemAdmissionDigest    authority.Digest
	systemAdmissionCanonical []byte
	roleAdmissionRef         ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef
	roleAdmissionDigest      authority.Digest
	roleAdmissionCanonical   []byte
	justificationRef         ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef
	justificationDigest      authority.Digest
	justificationCanonical   []byte
	provenanceRef            ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef
	provenanceDigest         authority.Digest
	provenanceCanonical      []byte
}

type executionRoleAssignmentCoordinates struct {
	project       projectidentity.ProjectID
	context       authority.BoundedContextRef
	window        authority.TimeWindow
	description   authority.DescriptionRef
	contentDigest authority.Digest
}

func executionRoleAssignmentCoordinatesFromContent(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) (executionRoleAssignmentCoordinates, error) {
	if err := content.Verify(); err != nil {
		return executionRoleAssignmentCoordinates{}, err
	}
	return executionRoleAssignmentCoordinates{
		project:       content.Project(),
		context:       content.JudgementContext(),
		window:        content.ValidityWindow(),
		description:   content.DescriptionRef(),
		contentDigest: content.Digest(),
	}, nil
}

func sealExecutionRoleAssignmentSupport(
	policy ProjectTypeEnvHeadSelectionExecutionRolePolicy,
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) (executionRoleAssignmentSupport, error) {
	coordinates, err := executionRoleAssignmentCoordinatesFromContent(content)
	if err != nil {
		return executionRoleAssignmentSupport{}, err
	}
	if err := policy.VerifyCurrentForNewWrite(coordinates.window); err != nil {
		return executionRoleAssignmentSupport{}, err
	}
	return sealExecutionRoleAssignmentSupportForCoordinates(policy, coordinates)
}

func sealExecutionRoleAssignmentSupportForCoordinates(
	policy ProjectTypeEnvHeadSelectionExecutionRolePolicy,
	coordinates executionRoleAssignmentCoordinates,
) (executionRoleAssignmentSupport, error) {
	systemRef, systemDigest, systemCanonical, err := sealExecutionSystemAdmission(
		policy,
		coordinates,
	)
	if err != nil {
		return executionRoleAssignmentSupport{}, err
	}
	roleRef, roleDigest, roleCanonical, err := sealExecutionRoleAdmission(
		policy,
		coordinates,
	)
	if err != nil {
		return executionRoleAssignmentSupport{}, err
	}
	justificationRef, justificationDigest, justificationCanonical, err := sealExecutionAssignmentJustification(
		policy,
		coordinates,
		systemRef,
		systemDigest,
		roleRef,
		roleDigest,
	)
	if err != nil {
		return executionRoleAssignmentSupport{}, err
	}
	provenanceRef, provenanceDigest, provenanceCanonical, err := sealExecutionAssignmentProvenance(
		policy,
		coordinates,
		justificationRef,
		justificationDigest,
	)
	if err != nil {
		return executionRoleAssignmentSupport{}, err
	}
	return executionRoleAssignmentSupport{
		systemAdmissionRef:       systemRef,
		systemAdmissionDigest:    systemDigest,
		systemAdmissionCanonical: systemCanonical,
		roleAdmissionRef:         roleRef,
		roleAdmissionDigest:      roleDigest,
		roleAdmissionCanonical:   roleCanonical,
		justificationRef:         justificationRef,
		justificationDigest:      justificationDigest,
		justificationCanonical:   justificationCanonical,
		provenanceRef:            provenanceRef,
		provenanceDigest:         provenanceDigest,
		provenanceCanonical:      provenanceCanonical,
	}, nil
}

func sealExecutionSystemAdmission(
	policy ProjectTypeEnvHeadSelectionExecutionRolePolicy,
	coordinates executionRoleAssignmentCoordinates,
) (
	ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef,
	authority.Digest,
	[]byte,
	error,
) {
	projection := executionSystemAdmissionProjection{
		Schema:                executionSystemAdmissionSchema,
		Project:               coordinates.project.String(),
		SystemRef:             policy.HolderSystem().String(),
		AdmittedSystemKind:    "U.System",
		BoundedContextRef:     coordinates.context.String(),
		ValidFrom:             formatTime(coordinates.window.From()),
		ValidUntil:            formatTime(coordinates.window.Until()),
		GoverningPatternRef:   executionRoleSystemPattern,
		PolicyRef:             policy.Ref().String(),
		PolicyDigest:          policy.Digest().String(),
		PolicyEditionRef:      policy.Edition().String(),
		ProductContractDigest: policy.ContractDigest().String(),
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef{}, authority.Digest{}, nil, err
	}
	digest, err := digestCanonical(executionSystemAdmissionDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef{}, authority.Digest{}, nil, err
	}
	return ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef{
		digest: digest,
	}, digest, canonical, nil
}

func sealExecutionRoleAdmission(
	policy ProjectTypeEnvHeadSelectionExecutionRolePolicy,
	coordinates executionRoleAssignmentCoordinates,
) (
	ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef,
	authority.Digest,
	[]byte,
	error,
) {
	projection := executionRoleAdmissionProjection{
		Schema:                executionRoleAdmissionSchema,
		Project:               coordinates.project.String(),
		RoleRef:               policy.Role().String(),
		BoundedContextRef:     coordinates.context.String(),
		ValidFrom:             formatTime(coordinates.window.From()),
		ValidUntil:            formatTime(coordinates.window.Until()),
		GoverningPatternRef:   executionRolePattern,
		PolicyRef:             policy.Ref().String(),
		PolicyDigest:          policy.Digest().String(),
		PolicyEditionRef:      policy.Edition().String(),
		ProductContractDigest: policy.ContractDigest().String(),
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef{}, authority.Digest{}, nil, err
	}
	digest, err := digestCanonical(executionRoleAdmissionDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef{}, authority.Digest{}, nil, err
	}
	return ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef{
		digest: digest,
	}, digest, canonical, nil
}

func sealExecutionAssignmentJustification(
	policy ProjectTypeEnvHeadSelectionExecutionRolePolicy,
	coordinates executionRoleAssignmentCoordinates,
	systemRef ProjectTypeEnvHeadSelectionExecutionSystemAdmissionRef,
	systemDigest authority.Digest,
	roleRef ProjectTypeEnvHeadSelectionExecutionRoleAdmissionRef,
	roleDigest authority.Digest,
) (
	ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef,
	authority.Digest,
	[]byte,
	error,
) {
	projection := executionAssignmentJustificationProjection{
		Schema:                executionAssignmentJustificationSchema,
		Project:               coordinates.project.String(),
		GoverningPatternRef:   executionRoleAssignmentPattern,
		RuleRef:               executionAssignmentRuleRefV1,
		Rule:                  executionAssignmentRuleV1,
		BoundedContextRef:     coordinates.context.String(),
		AssignmentFrom:        formatTime(coordinates.window.From()),
		AssignmentUntil:       formatTime(coordinates.window.Until()),
		SystemAdmissionRef:    systemRef.String(),
		SystemAdmissionDigest: systemDigest.String(),
		RoleAdmissionRef:      roleRef.String(),
		RoleAdmissionDigest:   roleDigest.String(),
		PolicyRef:             policy.Ref().String(),
		PolicyDigest:          policy.Digest().String(),
		PolicyEditionRef:      policy.Edition().String(),
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef{}, authority.Digest{}, nil, err
	}
	digest, err := digestCanonical(executionAssignmentJustificationDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef{}, authority.Digest{}, nil, err
	}
	return ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef{
		digest: digest,
	}, digest, canonical, nil
}

func sealExecutionAssignmentProvenance(
	policy ProjectTypeEnvHeadSelectionExecutionRolePolicy,
	coordinates executionRoleAssignmentCoordinates,
	justificationRef ProjectTypeEnvHeadSelectionExecutionAssignmentJustificationRef,
	justificationDigest authority.Digest,
) (
	ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef,
	authority.Digest,
	[]byte,
	error,
) {
	policyProjection := executionRolePolicyProjection{}
	if err := decodeStrictJSON(policy.CanonicalJSON(), &policyProjection); err != nil {
		return ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef{}, authority.Digest{}, nil, err
	}
	projection := executionAssignmentProvenanceProjection{
		Schema:                       executionAssignmentProvenanceSchema,
		Project:                      coordinates.project.String(),
		JustificationRef:             justificationRef.String(),
		JustificationDigest:          justificationDigest.String(),
		AuthorizationDescriptionKind: string(coordinates.description.Kind()),
		AuthorizationDescriptionRef:  coordinates.description.String(),
		AuthorizationContentDigest:   coordinates.contentDigest.String(),
		PolicyRef:                    policy.Ref().String(),
		PolicyDigest:                 policy.Digest().String(),
		PolicyEditionRef:             policy.Edition().String(),
		ProductContractDigest:        policy.ContractDigest().String(),
		SourcePins:                   policyProjection.SourcePins,
		Derivation:                   "sealed_from_registered_policy_and_exact_authorization_content",
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef{}, authority.Digest{}, nil, err
	}
	digest, err := digestCanonical(executionAssignmentProvenanceDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef{}, authority.Digest{}, nil, err
	}
	return ProjectTypeEnvHeadSelectionExecutionAssignmentProvenanceRef{
		digest: digest,
	}, digest, canonical, nil
}

func rebuildExecutionRoleAssignmentSupport(
	edition string,
	project projectidentity.ProjectID,
	context authority.BoundedContextRef,
	window authority.TimeWindow,
	description authority.DescriptionRef,
	contentDigest authority.Digest,
) (executionRoleAssignmentSupport, error) {
	policy, err := projectTypeEnvHeadSelectionExecutionRolePolicyForEdition(edition)
	if err != nil {
		return executionRoleAssignmentSupport{}, err
	}
	coordinates := executionRoleAssignmentCoordinates{
		project:       project,
		context:       context,
		window:        window,
		description:   description,
		contentDigest: contentDigest,
	}
	return sealExecutionRoleAssignmentSupportForCoordinates(policy, coordinates)
}
