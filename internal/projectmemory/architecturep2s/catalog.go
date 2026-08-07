package architecturep2s

import "fmt"

type positionSourceSpec struct {
	patternID       string
	returnCondition string
}

var positionSourceSpecs = map[PositionKind]positionSourceSpec{
	PositionHolonPosture: {
		patternID:       "A.1",
		returnCondition: "supply the exact constructive holon-recognition basis",
	},
	PositionProblemPressure: {
		patternID:       "C.22.PFR",
		returnCondition: "supply a direct ProblematicForRelation under an applicable criterion",
	},
	PositionArchitectureDescription: {
		patternID:       "C.30",
		returnCondition: "supply a direct architecture-description claim for this EntityOfConcern and context",
	},
	PositionSelectedStructure: {
		patternID:       "C.32.P2S",
		returnCondition: "supply a direct selected-structure claim without promoting a description or decision",
	},
	PositionArchitectureCandidate: {
		patternID:       "C.30",
		returnCondition: "supply a directly admitted architecture candidate with its bounded use and losses",
	},
	PositionAlternatives: {
		patternID:       "C.32.P2S",
		returnCondition: "supply an exact concern-scoped alternatives relation",
	},
	PositionComparison: {
		patternID:       "C.32.P2S",
		returnCondition: "supply a direct comparison claim with its exact comparison basis",
	},
	PositionDecision: {
		patternID:       "C.32.P2S",
		returnCondition: "supply a direct recorded-choice relation; a recommendation is insufficient",
	},
	PositionExpectedStructure: {
		patternID:       "C.30",
		returnCondition: "supply a direct expected-structure claim distinct from selected and actual structure",
	},
	PositionWorkRecord: {
		patternID:       "A.15.1",
		returnCondition: "supply an exact Work-record relation while keeping the record distinct from Work",
	},
	PositionPerformedWork: {
		patternID:       "A.15.1",
		returnCondition: "supply an identified dated Work occurrence and its exact identity basis",
	},
	PositionActualChange: {
		patternID:       "A.3.4",
		returnCondition: "supply the exact bounded actual-change basis and continuity condition",
	},
	PositionWorkToChange: {
		patternID:       "A.6.P.WMR",
		returnCondition: "supply the direct governed work-to-change claim with subject, object, modality, polarity, and support posture",
	},
	PositionProductionWork: {
		patternID:       "A.15.PROD",
		returnCondition: "supply the production-work participation branch under its own direct governor",
	},
	PositionEntityInception: {
		patternID:       "A.15.PROD",
		returnCondition: "supply the entity-identity inception branch and continuity basis",
	},
	PositionProductionCompletion: {
		patternID:       "A.15.PROD",
		returnCondition: "supply the historically indexed production-completion branch",
	},
	PositionActualStructure: {
		patternID:       "C.30",
		returnCondition: "supply actual subject-side organization facts over the declared substrate",
	},
	PositionStructureDescription: {
		patternID:       "C.30",
		returnCondition: "supply the direct structure-description relation without asserting actuality",
	},
	PositionStructureEvaluation: {
		patternID:       "C.30",
		returnCondition: "supply a criterion-bound structure evaluation distinct from structure and description",
	},
	PositionGrounding: {
		patternID:       "E.18.1",
		returnCondition: "supply the exact grounding relation and receiving use",
	},
	PositionEvidence: {
		patternID:       "A.10",
		returnCondition: "supply a governed evidence-use relation; a successful command is insufficient",
	},
	PositionConformance: {
		patternID:       "C.30",
		returnCondition: "supply an applicable criterion and direct conformance result",
	},
	PositionTargetEffect: {
		patternID:       "E.18.1",
		returnCondition: "supply the exact target, criterion, relation, applicability, and observation basis",
	},
}

func positionSourceReturn(kind PositionKind) (SourceReturn, error) {
	spec, found := positionSourceSpecs[kind]
	if !found {
		return SourceReturn{}, fmt.Errorf(
			"architecture P2S position %q has no source-return contract",
			kind,
		)
	}
	return NewSourceReturn(spec.patternID, spec.returnCondition)
}

// HaftV9RuleSet maps only already-existing local claims. It does not declare a
// new relation kind or claim that these local records are exact FPF kinds.
// Record-like relations are source docks unless their own exact local meaning
// is precisely the requested read position.
func HaftV9RuleSet() RuleSet {
	claimInputs := []ClaimRuleInput{
		{
			Position:  PositionAlternatives,
			Signature: "Haft.SolutionPortfolioAtConcern",
			PatternID: "C.32.P2S",
		},
		{
			Position:  PositionComparison,
			Signature: "Haft.PortfolioComparison",
			PatternID: "C.32.P2S",
		},
		{
			Position:  PositionDecision,
			Signature: "Haft.DecisionChoiceAtConcern",
			PatternID: "C.32.P2S",
		},
		{
			Position:  PositionWorkRecord,
			Signature: "Haft.WorkOccurrenceRecord",
			PatternID: "A.15.1",
		},
		{
			Position:  PositionWorkToChange,
			Signature: "Haft.CodeChangedByWork",
			PatternID: "A.6.P.WMR",
		},
		{
			Position:  PositionEvidence,
			Signature: "Haft.EvidenceUse",
			PatternID: "A.10",
		},
	}
	dockInputs := []SourceDockRuleInput{
		{Position: PositionProblemPressure, Signature: "Haft.ProblemCardAtConcern"},
		{Position: PositionArchitectureDescription, Signature: "Haft.SpecSectionAtConcern"},
		{Position: PositionSelectedStructure, Signature: "Haft.SpecSectionAtConcern"},
		{Position: PositionArchitectureCandidate, Signature: "Haft.SolutionPortfolioAtConcern"},
		{Position: PositionExpectedStructure, Signature: "Haft.SpecSectionAtConcern"},
		{Position: PositionPerformedWork, Signature: "Haft.WorkOccurrenceRecord"},
		{Position: PositionActualChange, Signature: "Haft.CodeChangedByWork"},
		{Position: PositionActualStructure, Signature: "Haft.CodeChangedByWork"},
		{Position: PositionStructureDescription, Signature: "Haft.SpecSectionAtConcern"},
		{Position: PositionStructureEvaluation, Signature: "Haft.EvidenceUse"},
		{Position: PositionGrounding, Signature: "Haft.EvidenceUse"},
		{Position: PositionConformance, Signature: "Haft.EvidenceUse"},
		{Position: PositionTargetEffect, Signature: "Haft.EvidenceUse"},
	}
	claimRules := make([]ClaimRule, 0, len(claimInputs))
	for _, input := range claimInputs {
		rule, err := NewClaimRule(input)
		if err != nil {
			panic(err)
		}
		claimRules = append(claimRules, rule)
	}
	dockRules := make([]SourceDockRule, 0, len(dockInputs))
	for _, input := range dockInputs {
		rule, err := NewSourceDockRule(input)
		if err != nil {
			panic(err)
		}
		dockRules = append(dockRules, rule)
	}
	rules, err := NewRuleSet(claimRules, dockRules)
	if err != nil {
		panic(err)
	}
	return rules
}
