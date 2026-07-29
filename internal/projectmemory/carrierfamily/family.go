package carrierfamily

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type familyV1 uint8

const (
	carrierEditionFamilyV1 familyV1 = iota + 1
	projectClaimFamilyV1
	performedWorkOccurrenceFamilyV1
	codeAnchorFamilyV1
)

func (family familyV1) token() string {
	switch family {
	case carrierEditionFamilyV1:
		return "carrier_edition"
	case projectClaimFamilyV1:
		return "project_claim"
	case performedWorkOccurrenceFamilyV1:
		return "performed_work_occurrence"
	case codeAnchorFamilyV1:
		return "code_anchor"
	default:
		return ""
	}
}

func (family familyV1) ruleRaw() string {
	switch family {
	case carrierEditionFamilyV1:
		return "haft.member-of.carrier-edition-carrier/v1"
	case projectClaimFamilyV1:
		return "haft.member-of.project-claim-carrier/v1"
	case performedWorkOccurrenceFamilyV1:
		return "haft.member-of.performed-work-occurrence-carrier/v1"
	case codeAnchorFamilyV1:
		return "haft.member-of.code-anchor-carrier/v1"
	default:
		return ""
	}
}

func (family familyV1) rule() (typedmemory.RuleRef, error) {
	raw := family.ruleRaw()
	rule, err := typedmemory.NewRuleRef(raw)
	if err != nil || rule.String() != raw {
		return typedmemory.RuleRef{}, fmt.Errorf("carrier-family evaluator rule is invalid")
	}
	return rule, nil
}

func parseFamilyV1(raw string) (familyV1, error) {
	values := []familyV1{
		carrierEditionFamilyV1,
		projectClaimFamilyV1,
		performedWorkOccurrenceFamilyV1,
		codeAnchorFamilyV1,
	}
	for _, value := range values {
		if value.token() == raw {
			return value, nil
		}
	}
	return 0, fmt.Errorf("unsupported carrier-family token %q", raw)
}

func CarrierEditionEvaluatorRuleV1() typedmemory.RuleRef {
	rule, _ := carrierEditionFamilyV1.rule()
	return rule
}

func ProjectClaimEvaluatorRuleV1() typedmemory.RuleRef {
	rule, _ := projectClaimFamilyV1.rule()
	return rule
}

func PerformedWorkOccurrenceEvaluatorRuleV1() typedmemory.RuleRef {
	rule, _ := performedWorkOccurrenceFamilyV1.rule()
	return rule
}

func CodeAnchorEvaluatorRuleV1() typedmemory.RuleRef {
	rule, _ := codeAnchorFamilyV1.rule()
	return rule
}
