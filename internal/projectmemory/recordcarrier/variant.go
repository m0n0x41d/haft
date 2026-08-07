package recordcarrier

import "fmt"

// ProjectRecordCarrierVariantV1 is the closed carrier-classification union.
// It is deliberately not a caller-supplied U.Kind/Haft.Kind identifier.
// Note, ProblemCard, SolutionPortfolio, and comparison remain relation roles,
// not variants of this carrier contract.
type ProjectRecordCarrierVariantV1 interface {
	Token() string
	projectRecordCarrierVariantV1()
}

type GenericProjectRecordVariantV1 struct{}

func (GenericProjectRecordVariantV1) Token() string { return "project_record" }

func (GenericProjectRecordVariantV1) projectRecordCarrierVariantV1() {}

type DecisionRecordVariantV1 struct{}

func (DecisionRecordVariantV1) Token() string { return "decision_record" }

func (DecisionRecordVariantV1) projectRecordCarrierVariantV1() {}

type SpecSectionRecordVariantV1 struct{}

func (SpecSectionRecordVariantV1) Token() string { return "spec_section_record" }

func (SpecSectionRecordVariantV1) projectRecordCarrierVariantV1() {}

type EvidenceRecordVariantV1 struct{}

func (EvidenceRecordVariantV1) Token() string { return "evidence_record" }

func (EvidenceRecordVariantV1) projectRecordCarrierVariantV1() {}

type SupportingEpistemeRecordVariantV1 struct{}

func (SupportingEpistemeRecordVariantV1) Token() string {
	return "supporting_episteme_record"
}

func (SupportingEpistemeRecordVariantV1) projectRecordCarrierVariantV1() {}

type WorkRecordVariantV1 struct{}

func (WorkRecordVariantV1) Token() string { return "work_record" }

func (WorkRecordVariantV1) projectRecordCarrierVariantV1() {}

type WorkPlanRecordVariantV1 struct{}

func (WorkPlanRecordVariantV1) Token() string { return "work_plan_record" }

func (WorkPlanRecordVariantV1) projectRecordCarrierVariantV1() {}

func parseProjectRecordCarrierVariantV1(
	raw string,
) (ProjectRecordCarrierVariantV1, error) {
	switch raw {
	case GenericProjectRecordVariantV1{}.Token():
		return GenericProjectRecordVariantV1{}, nil
	case DecisionRecordVariantV1{}.Token():
		return DecisionRecordVariantV1{}, nil
	case SpecSectionRecordVariantV1{}.Token():
		return SpecSectionRecordVariantV1{}, nil
	case EvidenceRecordVariantV1{}.Token():
		return EvidenceRecordVariantV1{}, nil
	case SupportingEpistemeRecordVariantV1{}.Token():
		return SupportingEpistemeRecordVariantV1{}, nil
	case WorkRecordVariantV1{}.Token():
		return WorkRecordVariantV1{}, nil
	case WorkPlanRecordVariantV1{}.Token():
		return WorkPlanRecordVariantV1{}, nil
	default:
		return nil, fmt.Errorf("unsupported project-record carrier variant %q", raw)
	}
}

func validateProjectRecordCarrierVariantV1(
	variant ProjectRecordCarrierVariantV1,
) error {
	switch variant.(type) {
	case GenericProjectRecordVariantV1,
		DecisionRecordVariantV1,
		SpecSectionRecordVariantV1,
		EvidenceRecordVariantV1,
		SupportingEpistemeRecordVariantV1,
		WorkRecordVariantV1,
		WorkPlanRecordVariantV1:
		return nil
	default:
		return fmt.Errorf("project-record carrier variant is required or unsupported")
	}
}

func sameProjectRecordCarrierVariantV1(
	left ProjectRecordCarrierVariantV1,
	right ProjectRecordCarrierVariantV1,
) bool {
	if validateProjectRecordCarrierVariantV1(left) != nil {
		return false
	}
	if validateProjectRecordCarrierVariantV1(right) != nil {
		return false
	}
	return left.Token() == right.Token()
}
