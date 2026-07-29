package projecttypeenvruntime

import "sort"

// IssueKind separates malformed input, unavailable implementation, and
// observed identity drift. These postures are intentionally not coercible.
type IssueKind uint8

const (
	IssueKindInvalid IssueKind = iota + 1
	IssueKindUnavailable
	IssueKindDrift
)

func (kind IssueKind) String() string {
	switch kind {
	case IssueKindInvalid:
		return "invalid"
	case IssueKindUnavailable:
		return "unavailable"
	case IssueKindDrift:
		return "drift"
	default:
		return ""
	}
}

// IssueCode is the stable machine-facing runtime-registry failure reason.
type IssueCode string

const (
	IssueRuntimeBasisInvalid          IssueCode = "runtime_basis_invalid"
	IssueMechanismCatalogInvalid      IssueCode = "mechanism_catalog_invalid"
	IssueRegistrationPolicyInvalid    IssueCode = "registration_policy_invalid"
	IssueMechanismCatalogDuplicate    IssueCode = "mechanism_catalog_duplicate"
	IssueRegistrationPolicyDuplicate  IssueCode = "registration_policy_duplicate"
	IssueMechanismCatalogMissing      IssueCode = "mechanism_catalog_missing"
	IssueCodecImplementationMissing   IssueCode = "codec_implementation_missing"
	IssueEvaluatorRegistrationMissing IssueCode = "evaluator_registration_missing"
	IssueEvaluatorContractUnsupported IssueCode = "evaluator_contract_unsupported"
	IssueRegistrationPolicyMissing    IssueCode = "registration_policy_missing"
	IssueMechanismCatalogDrift        IssueCode = "mechanism_catalog_identity_mismatch"
	IssueMechanismCatalogEntryDrift   IssueCode = "mechanism_catalog_entry_mismatch"
	IssueEvaluatorIdentityDrift       IssueCode = "evaluator_identity_mismatch"
	IssueRegistrationPolicyDrift      IssueCode = "registration_policy_identity_mismatch"
	IssuePolicyEvaluatorDrift         IssueCode = "registration_policy_evaluator_mismatch"
	IssuePolicySourceDeliveryDrift    IssueCode = "registration_policy_source_delivery_mismatch"
	IssueUnexpectedRegistrationPolicy IssueCode = "registration_policy_unexpected"
)

type issueMetadata struct {
	kind IssueKind
	rank int
}

func metadataForIssue(code IssueCode) issueMetadata {
	switch code {
	case IssueRuntimeBasisInvalid:
		return issueMetadata{kind: IssueKindInvalid, rank: 10}
	case IssueMechanismCatalogInvalid:
		return issueMetadata{kind: IssueKindInvalid, rank: 20}
	case IssueRegistrationPolicyInvalid:
		return issueMetadata{kind: IssueKindInvalid, rank: 30}
	case IssueMechanismCatalogDuplicate:
		return issueMetadata{kind: IssueKindUnavailable, rank: 100}
	case IssueRegistrationPolicyDuplicate:
		return issueMetadata{kind: IssueKindUnavailable, rank: 110}
	case IssueMechanismCatalogMissing:
		return issueMetadata{kind: IssueKindUnavailable, rank: 120}
	case IssueCodecImplementationMissing:
		return issueMetadata{kind: IssueKindUnavailable, rank: 130}
	case IssueEvaluatorRegistrationMissing:
		return issueMetadata{kind: IssueKindUnavailable, rank: 140}
	case IssueEvaluatorContractUnsupported:
		return issueMetadata{kind: IssueKindUnavailable, rank: 150}
	case IssueRegistrationPolicyMissing:
		return issueMetadata{kind: IssueKindUnavailable, rank: 160}
	case IssueMechanismCatalogDrift:
		return issueMetadata{kind: IssueKindDrift, rank: 200}
	case IssueMechanismCatalogEntryDrift:
		return issueMetadata{kind: IssueKindDrift, rank: 210}
	case IssueEvaluatorIdentityDrift:
		return issueMetadata{kind: IssueKindDrift, rank: 220}
	case IssueRegistrationPolicyDrift:
		return issueMetadata{kind: IssueKindDrift, rank: 230}
	case IssuePolicyEvaluatorDrift:
		return issueMetadata{kind: IssueKindDrift, rank: 240}
	case IssuePolicySourceDeliveryDrift:
		return issueMetadata{kind: IssueKindDrift, rank: 250}
	case IssueUnexpectedRegistrationPolicy:
		return issueMetadata{kind: IssueKindDrift, rank: 260}
	default:
		return issueMetadata{}
	}
}

// Issue is immutable diagnostic data. Expected and actual are exact display
// coordinates, not executable evidence or authority.
type Issue struct {
	kind     IssueKind
	code     IssueCode
	subject  string
	expected string
	actual   string
	repair   string
}

func (issue Issue) Kind() IssueKind { return issue.kind }

func (issue Issue) Code() IssueCode { return issue.code }

func (issue Issue) Subject() string { return issue.subject }

func (issue Issue) Expected() string { return issue.expected }

func (issue Issue) Actual() string { return issue.actual }

func (issue Issue) Repair() string { return issue.repair }

func newIssue(
	code IssueCode,
	subject string,
	expected string,
	actual string,
	repair string,
) Issue {
	metadata := metadataForIssue(code)
	return Issue{
		kind:     metadata.kind,
		code:     code,
		subject:  subject,
		expected: expected,
		actual:   actual,
		repair:   repair,
	}
}

func normalizeIssues(values []Issue) []Issue {
	owned := append([]Issue(nil), values...)
	sort.Slice(owned, func(left int, right int) bool {
		leftMetadata := metadataForIssue(owned[left].code)
		rightMetadata := metadataForIssue(owned[right].code)
		if leftMetadata.rank != rightMetadata.rank {
			return leftMetadata.rank < rightMetadata.rank
		}
		if owned[left].kind != owned[right].kind {
			return owned[left].kind < owned[right].kind
		}
		if owned[left].code != owned[right].code {
			return owned[left].code < owned[right].code
		}
		if owned[left].subject != owned[right].subject {
			return owned[left].subject < owned[right].subject
		}
		if owned[left].expected != owned[right].expected {
			return owned[left].expected < owned[right].expected
		}
		if owned[left].actual != owned[right].actual {
			return owned[left].actual < owned[right].actual
		}
		return owned[left].repair < owned[right].repair
	})
	result := make([]Issue, 0, len(owned))
	for _, issue := range owned {
		if len(result) > 0 && equalIssues(result[len(result)-1], issue) {
			continue
		}
		result = append(result, issue)
	}
	return result
}

func equalIssues(left Issue, right Issue) bool {
	return left.kind == right.kind &&
		left.code == right.code &&
		left.subject == right.subject &&
		left.expected == right.expected &&
		left.actual == right.actual &&
		left.repair == right.repair
}

func containsIssueKind(issues []Issue, kind IssueKind) bool {
	for _, issue := range issues {
		if issue.kind == kind {
			return true
		}
	}
	return false
}
