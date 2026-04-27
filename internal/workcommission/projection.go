package workcommission

import "strings"

type ProjectionPolicy string

const (
	ProjectionPolicyLocalOnly        ProjectionPolicy = "local_only"
	ProjectionPolicyExternalOptional ProjectionPolicy = "external_optional"
	ProjectionPolicyExternalRequired ProjectionPolicy = "external_required"
)

type ProjectionClaimKind string

const (
	ProjectionClaimStatus     ProjectionClaimKind = "status"
	ProjectionClaimOwner      ProjectionClaimKind = "owner"
	ProjectionClaimDate       ProjectionClaimKind = "date"
	ProjectionClaimSeverity   ProjectionClaimKind = "severity"
	ProjectionClaimCompletion ProjectionClaimKind = "completion"
	ProjectionClaimScope      ProjectionClaimKind = "scope"
	ProjectionClaimPromise    ProjectionClaimKind = "promise"
)

type ProjectionClaim struct {
	Kind  ProjectionClaimKind
	Value string
}

type ProjectionIntent struct {
	RequiredClaims []ProjectionClaim
	OptionalClaims []ProjectionClaim
	ForbiddenKinds []ProjectionClaimKind
	RequiredLinks  []string
}

type ProjectionDraft struct {
	Claims []ProjectionClaim
	Links  []string
}

type ProjectionValidationVerdict string

const (
	ProjectionValidationPass   ProjectionValidationVerdict = "pass"
	ProjectionValidationReject ProjectionValidationVerdict = "reject"
)

type ProjectionValidationCode string

const (
	ProjectionValidationInvalidClaim   ProjectionValidationCode = "invalid_claim"
	ProjectionValidationInventedClaim  ProjectionValidationCode = "invented_claim"
	ProjectionValidationChangedClaim   ProjectionValidationCode = "changed_claim"
	ProjectionValidationForbiddenClaim ProjectionValidationCode = "forbidden_claim"
	ProjectionValidationMissingClaim   ProjectionValidationCode = "missing_claim"
	ProjectionValidationMissingLink    ProjectionValidationCode = "missing_link"
)

type ProjectionValidationIssue struct {
	Code   ProjectionValidationCode
	Kind   ProjectionClaimKind
	Detail string
}

type ProjectionValidation struct {
	Verdict ProjectionValidationVerdict
	Issues  []ProjectionValidationIssue
}

type ProjectionPublicationState string

const (
	ProjectionPublicationMissing ProjectionPublicationState = "missing"
	ProjectionPublicationFailed  ProjectionPublicationState = "failed"
	ProjectionPublicationSynced  ProjectionPublicationState = "synced"
)

type ProjectionPublication struct {
	State       ProjectionPublicationState
	Carrier     string
	Target      string
	LastError   string
	RetryPolicy string
}

type ProjectionDebt struct {
	Carrier     string `json:"carrier"`
	Target      string `json:"target"`
	LastError   string `json:"last_error"`
	RetryPolicy string `json:"retry_policy"`
}

type ProjectionCompletion struct {
	State State
	Debt  *ProjectionDebt
}

var knownProjectionClaimKinds = map[ProjectionClaimKind]struct{}{
	ProjectionClaimStatus:     {},
	ProjectionClaimOwner:      {},
	ProjectionClaimDate:       {},
	ProjectionClaimSeverity:   {},
	ProjectionClaimCompletion: {},
	ProjectionClaimScope:      {},
	ProjectionClaimPromise:    {},
}

func AuthorityProjectionClaimKinds() []ProjectionClaimKind {
	return []ProjectionClaimKind{
		ProjectionClaimStatus,
		ProjectionClaimOwner,
		ProjectionClaimDate,
		ProjectionClaimSeverity,
		ProjectionClaimCompletion,
		ProjectionClaimScope,
		ProjectionClaimPromise,
	}
}

func KnownProjectionClaimKind(kind ProjectionClaimKind) bool {
	_, ok := knownProjectionClaimKinds[kind]
	return ok
}

func ValidateProjectionDraft(intent ProjectionIntent, draft ProjectionDraft) ProjectionValidation {
	index := projectionIntentIndex(intent)
	issues := projectionDraftClaimIssues(index, draft.Claims)
	issues = append(issues, projectionMissingClaimIssues(index, draft.Claims)...)
	issues = append(issues, projectionMissingLinkIssues(intent.RequiredLinks, draft.Links)...)

	if len(issues) > 0 {
		return ProjectionValidation{
			Verdict: ProjectionValidationReject,
			Issues:  issues,
		}
	}

	return ProjectionValidation{Verdict: ProjectionValidationPass}
}

func CompletionAfterLocalEvidence(
	policy ProjectionPolicy,
	publication ProjectionPublication,
) ProjectionCompletion {
	if policy != ProjectionPolicyExternalRequired {
		return ProjectionCompletion{State: StateCompleted}
	}

	if publication.State == ProjectionPublicationSynced {
		return ProjectionCompletion{State: StateCompleted}
	}

	debt := projectionDebt(publication)
	return ProjectionCompletion{
		State: StateCompletedWithProjectionDebt,
		Debt:  &debt,
	}
}

func NormalizeProjectionPolicy(value string) ProjectionPolicy {
	switch ProjectionPolicy(strings.TrimSpace(value)) {
	case ProjectionPolicyExternalOptional:
		return ProjectionPolicyExternalOptional
	case ProjectionPolicyExternalRequired:
		return ProjectionPolicyExternalRequired
	default:
		return ProjectionPolicyLocalOnly
	}
}

func NormalizeProjectionPublicationState(value string) ProjectionPublicationState {
	switch ProjectionPublicationState(strings.TrimSpace(value)) {
	case ProjectionPublicationSynced:
		return ProjectionPublicationSynced
	case ProjectionPublicationFailed:
		return ProjectionPublicationFailed
	default:
		return ProjectionPublicationMissing
	}
}

type projectionClaimIndex struct {
	Allowed   map[ProjectionClaimKind]string
	Required  map[ProjectionClaimKind]string
	Forbidden map[ProjectionClaimKind]struct{}
}

func projectionIntentIndex(intent ProjectionIntent) projectionClaimIndex {
	allowed := make(map[ProjectionClaimKind]string)
	required := make(map[ProjectionClaimKind]string)
	forbidden := make(map[ProjectionClaimKind]struct{})

	for _, claim := range intent.OptionalClaims {
		allowed[claim.Kind] = strings.TrimSpace(claim.Value)
	}
	for _, claim := range intent.RequiredClaims {
		value := strings.TrimSpace(claim.Value)
		allowed[claim.Kind] = value
		required[claim.Kind] = value
	}
	for _, kind := range intent.ForbiddenKinds {
		forbidden[kind] = struct{}{}
	}

	return projectionClaimIndex{
		Allowed:   allowed,
		Required:  required,
		Forbidden: forbidden,
	}
}

func projectionDraftClaimIssues(
	index projectionClaimIndex,
	claims []ProjectionClaim,
) []ProjectionValidationIssue {
	issues := make([]ProjectionValidationIssue, 0)

	for _, claim := range claims {
		kind := claim.Kind
		value := strings.TrimSpace(claim.Value)

		if !KnownProjectionClaimKind(kind) {
			issues = append(issues, projectionIssue(ProjectionValidationInvalidClaim, kind, "claim kind is not in the closed projection schema"))
			continue
		}

		if _, ok := index.Forbidden[kind]; ok {
			issues = append(issues, projectionIssue(ProjectionValidationForbiddenClaim, kind, "draft includes a claim the intent forbids"))
			continue
		}

		expected, ok := index.Allowed[kind]
		if !ok {
			issues = append(issues, projectionIssue(ProjectionValidationInventedClaim, kind, "draft includes a claim absent from the deterministic intent"))
			continue
		}

		if value != expected {
			issues = append(issues, projectionIssue(ProjectionValidationChangedClaim, kind, "draft changed the deterministic intent value"))
		}
	}

	return issues
}

func projectionMissingClaimIssues(
	index projectionClaimIndex,
	claims []ProjectionClaim,
) []ProjectionValidationIssue {
	present := make(map[ProjectionClaimKind]string, len(claims))
	for _, claim := range claims {
		present[claim.Kind] = strings.TrimSpace(claim.Value)
	}

	issues := make([]ProjectionValidationIssue, 0)
	for kind, expected := range index.Required {
		value, ok := present[kind]
		if ok && value == expected {
			continue
		}

		issues = append(issues, projectionIssue(ProjectionValidationMissingClaim, kind, "draft omitted a required deterministic claim"))
	}

	return issues
}

func projectionMissingLinkIssues(requiredLinks []string, draftLinks []string) []ProjectionValidationIssue {
	present := make(map[string]struct{}, len(draftLinks))
	for _, link := range draftLinks {
		clean := strings.TrimSpace(link)
		if clean == "" {
			continue
		}

		present[clean] = struct{}{}
	}

	issues := make([]ProjectionValidationIssue, 0)
	for _, link := range requiredLinks {
		clean := strings.TrimSpace(link)
		if clean == "" {
			continue
		}

		if _, ok := present[clean]; ok {
			continue
		}

		issues = append(issues, ProjectionValidationIssue{
			Code:   ProjectionValidationMissingLink,
			Detail: clean,
		})
	}

	return issues
}

func projectionIssue(
	code ProjectionValidationCode,
	kind ProjectionClaimKind,
	detail string,
) ProjectionValidationIssue {
	return ProjectionValidationIssue{
		Code:   code,
		Kind:   kind,
		Detail: detail,
	}
}

func projectionDebt(publication ProjectionPublication) ProjectionDebt {
	return ProjectionDebt{
		Carrier:     projectionDebtCarrier(publication.Carrier),
		Target:      projectionDebtTarget(publication.Target),
		LastError:   projectionDebtLastError(publication),
		RetryPolicy: projectionDebtRetryPolicy(publication.RetryPolicy),
	}
}

func projectionDebtCarrier(value string) string {
	clean := strings.TrimSpace(value)
	if clean != "" {
		return clean
	}

	return "external_projection"
}

func projectionDebtTarget(value string) string {
	clean := strings.TrimSpace(value)
	if clean != "" {
		return clean
	}

	return "unconfigured_external_target"
}

func projectionDebtLastError(publication ProjectionPublication) string {
	clean := strings.TrimSpace(publication.LastError)
	if clean != "" {
		return clean
	}

	if publication.State == ProjectionPublicationFailed {
		return "external publication failed without a reported connector error"
	}

	return "external publication has not synced"
}

func projectionDebtRetryPolicy(value string) string {
	clean := strings.TrimSpace(value)
	if clean != "" {
		return clean
	}

	return "manual_or_connector_retry"
}
