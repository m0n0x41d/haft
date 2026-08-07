package projectmemory

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/kindclassificationruntime"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

const (
	projectEntityClassificationRule           = "haft.classify.project-entity/v1"
	projectRecordClassificationRule           = "haft.classify.project-record-carrier/v1"
	decisionRecordClassificationRule          = "haft.classify.decision-record-carrier/v1"
	specSectionRecordClassificationRule       = "haft.classify.spec-section-record-carrier/v1"
	evidenceRecordClassificationRule          = "haft.classify.evidence-record-carrier/v1"
	supportingEpistemeClassificationRule      = "haft.classify.supporting-episteme-record-carrier/v1"
	workRecordClassificationRule              = "haft.classify.work-record-carrier/v1"
	workPlanRecordClassificationRule          = "haft.classify.work-plan-record-carrier/v1"
	carrierEditionClassificationRule          = "haft.classify.carrier-edition-carrier/v1"
	projectClaimClassificationRule            = "haft.classify.project-claim-carrier/v1"
	performedWorkOccurrenceClassificationRule = "haft.classify.performed-work-occurrence-carrier/v1"
	codeAnchorClassificationRule              = "haft.classify.code-anchor-carrier/v1"
)

type projectKindClassificationCriterionSpec struct {
	localKind string
	key       string
	governor  string
	token     string
}

var projectKindClassificationCriteria = map[string]projectKindClassificationCriterionSpec{
	projectEntityClassificationRule: {
		localKind: "U.Entity",
		key:       entityPresenceFeatureKey,
		governor:  entityPresenceFeatureGovernor,
		token:     entityPresenceFeatureToken,
	},
	projectRecordClassificationRule: {
		localKind: "Haft.ProjectRecord",
		key:       projectObjectFamilyFeatureKey,
		governor:  projectObjectFamilyGovernor,
		token:     projectRecordFamilyToken,
	},
	decisionRecordClassificationRule: {
		localKind: "Haft.DecisionRecord",
		key:       projectRecordVariantFeatureKey,
		governor:  projectRecordVariantGovernor,
		token:     "decision_record",
	},
	specSectionRecordClassificationRule: {
		localKind: "Haft.SpecSectionRecord",
		key:       projectRecordVariantFeatureKey,
		governor:  projectRecordVariantGovernor,
		token:     "spec_section_record", // #nosec G101 -- domain classification token, not a credential.
	},
	evidenceRecordClassificationRule: {
		localKind: "Haft.EvidenceRecord",
		key:       projectRecordVariantFeatureKey,
		governor:  projectRecordVariantGovernor,
		token:     "evidence_record",
	},
	supportingEpistemeClassificationRule: {
		localKind: "Haft.SupportingEpistemeRecord",
		key:       projectRecordVariantFeatureKey,
		governor:  projectRecordVariantGovernor,
		token:     "supporting_episteme_record",
	},
	workRecordClassificationRule: {
		localKind: "Haft.WorkRecord",
		key:       projectRecordVariantFeatureKey,
		governor:  projectRecordVariantGovernor,
		token:     "work_record",
	},
	workPlanRecordClassificationRule: {
		localKind: "Haft.WorkPlanRecord",
		key:       projectRecordVariantFeatureKey,
		governor:  projectRecordVariantGovernor,
		token:     "work_plan_record",
	},
	carrierEditionClassificationRule: {
		localKind: "Haft.CarrierEdition",
		key:       projectObjectFamilyFeatureKey,
		governor:  projectObjectFamilyGovernor,
		token:     "carrier_edition",
	},
	projectClaimClassificationRule: {
		localKind: "Haft.ProjectClaim",
		key:       projectObjectFamilyFeatureKey,
		governor:  projectObjectFamilyGovernor,
		token:     "project_claim",
	},
	performedWorkOccurrenceClassificationRule: {
		localKind: "Haft.PerformedWorkOccurrence",
		key:       projectObjectFamilyFeatureKey,
		governor:  projectObjectFamilyGovernor,
		token:     "performed_work_occurrence",
	},
	codeAnchorClassificationRule: {
		localKind: "Haft.CodeAnchor",
		key:       projectObjectFamilyFeatureKey,
		governor:  projectObjectFamilyGovernor,
		token:     "code_anchor",
	},
}

// NewProjectKindClassificationEvaluatorRegistry binds only the reviewed
// current Local-Practice criteria. The TypeEnv supplies the exact declarations;
// this implementation supplies direct-feature predicates for those RuleRefs.
// Unknown or reassigned RuleRefs fail closed instead of becoming an implicit
// second classification language.
func NewProjectKindClassificationEvaluatorRegistry(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	identity typedmemoryevaluation.MechanismIdentity,
) (kindclassificationruntime.Registry, error) {
	definitions := environment.KindClassificationSignatureDefinitions()
	registrations := make(
		[]kindclassificationruntime.Registration,
		0,
		len(definitions),
	)
	for _, definition := range definitions {
		registration, err := projectKindClassificationRegistration(
			environment,
			codecs,
			identity,
			definition,
		)
		if err != nil {
			return kindclassificationruntime.Registry{}, err
		}
		registrations = append(registrations, registration)
	}
	registry, err := kindclassificationruntime.NewRegistry(registrations)
	if err != nil {
		return kindclassificationruntime.Registry{}, fmt.Errorf(
			"construct project kind-classification registry: %w",
			err,
		)
	}
	return registry, nil
}

func projectKindClassificationRegistration(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	identity typedmemoryevaluation.MechanismIdentity,
	definition typedmemory.KindClassificationSignatureDefinition,
) (kindclassificationruntime.Registration, error) {
	rule := definition.Criterion()
	spec, supported := projectKindClassificationCriteria[rule.String()]
	if !supported {
		return kindclassificationruntime.Registration{}, fmt.Errorf(
			"current KindSignature %q uses unsupported criterion RuleRef %q",
			definition.LocalKind().String(),
			rule.String(),
		)
	}
	localKind := definition.LocalKind().ValueKind().ID().String()
	if localKind != spec.localKind {
		return kindclassificationruntime.Registration{}, fmt.Errorf(
			"criterion RuleRef %q belongs to local kind %q, not %q",
			rule.String(),
			spec.localKind,
			localKind,
		)
	}
	candidateKind := definition.CandidateValueKind().ID().String()
	if candidateKind != "U.Entity" {
		return kindclassificationruntime.Registration{}, fmt.Errorf(
			"current KindSignature %q classifies %q, want U.Entity",
			definition.LocalKind().String(),
			candidateKind,
		)
	}
	predicate, err := projectKindClassificationPredicate(
		environment,
		codecs,
		spec,
	)
	if err != nil {
		return kindclassificationruntime.Registration{}, err
	}
	criterion, err := kindclassificationruntime.NewDirectFeatureCriterion(
		rule,
		[]kindclassificationruntime.DirectFeaturePredicate{predicate},
	)
	if err != nil {
		return kindclassificationruntime.Registration{}, fmt.Errorf(
			"construct criterion %q: %w",
			rule.String(),
			err,
		)
	}
	registration, err := kindclassificationruntime.NewRegistration(
		rule,
		identity,
		criterion,
	)
	if err != nil {
		return kindclassificationruntime.Registration{}, fmt.Errorf(
			"register criterion %q: %w",
			rule.String(),
			err,
		)
	}
	return registration, nil
}

func projectKindClassificationPredicate(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	spec projectKindClassificationCriterionSpec,
) (kindclassificationruntime.DirectFeaturePredicate, error) {
	key, err := typedmemory.NewKindFeatureKey(spec.key)
	if err != nil {
		return kindclassificationruntime.DirectFeaturePredicate{}, err
	}
	governor, err := typedmemory.NewRuleRef(spec.governor)
	if err != nil {
		return kindclassificationruntime.DirectFeaturePredicate{}, err
	}
	value, err := verifiedRecordFeatureText(environment, codecs, spec.token)
	if err != nil {
		return kindclassificationruntime.DirectFeaturePredicate{}, fmt.Errorf(
			"construct expected classification feature %q: %w",
			spec.key,
			err,
		)
	}
	predicate, err := kindclassificationruntime.NewDirectFeaturePredicate(
		kindclassificationruntime.DirectFeaturePredicateInput{
			Key:                 key,
			Governor:            governor,
			ExpectedValueKind:   value.ValueKind(),
			ExpectedValueDigest: value.Digest(),
		},
	)
	if err != nil {
		return kindclassificationruntime.DirectFeaturePredicate{}, err
	}
	return predicate, nil
}
