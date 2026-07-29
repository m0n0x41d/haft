package recordatconcern

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/recordmapping"
)

const (
	projectRecordKindID     = "Haft.ProjectRecord"
	projectRecordRefID      = "Haft.ProjectRecordRef"
	specSectionRecordKindID = "Haft.SpecSectionRecord"
	specSectionRecordRefID  = "Haft.SpecSectionRecordRef"
	entityKindID            = "U.Entity"
	entityRefID             = "U.EntityRef"
	claimGraphKindID        = "U.ClaimGraph"
)

type contractDefinition struct {
	manifestID             string
	manifestVersion        string
	adapterVersion         string
	signatureID            string
	recordSlotID           string
	recordKindID           string
	recordRefID            string
	recordVariant          string
	portfolioSlotID        string
	concernSlotID          string
	claimGraphSlotID       string
	optionSlotID           string
	comparedOptionSlotID   string
	nonDominatedSlotID     string
	problemSlotID          string
	chosenOptionSlotID     string
	rejectedOptionSlotID   string
	comparisonSlotID       string
	mappingRepair          string
	mappingRegistration    string
	registrationRepair     string
	claimGraphRepair       string
	relationDiagnosticName string
}

// Contract is one sealed record-at-concern mapping shape. Callers cannot
// manufacture arbitrary relation or slot coordinates: the package exposes
// only exact source-backed task constructors: Note, ProblemCard,
// SolutionPortfolio, PortfolioComparison, DecisionRecord projection, and
// SpecSection.
type Contract struct {
	definition contractDefinition
	manifest   recordmapping.MappingManifestRef
	adapter    recordmapping.AdapterVersion
}

func NewNoteContract(
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) (Contract, error) {
	definition := contractDefinition{
		manifestID:             "haft.note-at-concern",
		manifestVersion:        "2.0.0",
		adapterVersion:         "haft-note-adapter/2.0.0",
		signatureID:            "Haft.NoteAtConcern",
		recordSlotID:           "Haft.NoteAtConcern.NoteSlot",
		recordKindID:           projectRecordKindID,
		recordRefID:            projectRecordRefID,
		recordVariant:          (recordcarrier.GenericProjectRecordVariantV1{}).Token(),
		concernSlotID:          "Haft.NoteAtConcern.EntityOfConcernSlot",
		claimGraphSlotID:       "Haft.NoteAtConcern.ClaimGraphSlot",
		mappingRepair:          "repair:haft-note-mapping-manifest-v2",
		mappingRegistration:    "note_mapping_registration",
		registrationRepair:     "repair:select-runtime-accepting-note-mapping",
		claimGraphRepair:       "repair:provide-note-claim-graph",
		relationDiagnosticName: "note",
	}
	return newContract(definition, manifest, adapter)
}

func NewProblemCardContract(
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) (Contract, error) {
	definition := contractDefinition{
		manifestID:             "haft.problem-card-at-concern",
		manifestVersion:        "2.0.0",
		adapterVersion:         "haft-problem-card-adapter/2.0.0",
		signatureID:            "Haft.ProblemCardAtConcern",
		recordSlotID:           "Haft.ProblemCardAtConcern.ProblemCardSlot",
		recordKindID:           projectRecordKindID,
		recordRefID:            projectRecordRefID,
		recordVariant:          (recordcarrier.GenericProjectRecordVariantV1{}).Token(),
		concernSlotID:          "Haft.ProblemCardAtConcern.EntityOfConcernSlot",
		claimGraphSlotID:       "Haft.ProblemCardAtConcern.ClaimGraphSlot",
		mappingRepair:          "repair:haft-problem-card-mapping-manifest-v2",
		mappingRegistration:    "problem_card_mapping_registration",
		registrationRepair:     "repair:select-runtime-accepting-problem-card-mapping",
		claimGraphRepair:       "repair:provide-problem-card-claim-graph",
		relationDiagnosticName: "problem_card",
	}
	return newContract(definition, manifest, adapter)
}

func NewSolutionPortfolioContract(
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) (Contract, error) {
	definition := contractDefinition{
		manifestID:             "haft.solution-portfolio-at-concern",
		manifestVersion:        "2.0.0",
		adapterVersion:         "haft-solution-portfolio-adapter/2.0.0",
		signatureID:            "Haft.SolutionPortfolioAtConcern",
		recordSlotID:           "Haft.SolutionPortfolioAtConcern.PortfolioSlot",
		recordKindID:           projectRecordKindID,
		recordRefID:            projectRecordRefID,
		recordVariant:          (recordcarrier.GenericProjectRecordVariantV1{}).Token(),
		concernSlotID:          "Haft.SolutionPortfolioAtConcern.EntityOfConcernSlot",
		claimGraphSlotID:       "Haft.SolutionPortfolioAtConcern.ClaimGraphSlot",
		optionSlotID:           "Haft.SolutionPortfolioAtConcern.OptionSlot",
		mappingRepair:          "repair:haft-solution-portfolio-mapping-manifest-v2",
		mappingRegistration:    "solution_portfolio_mapping_registration",
		registrationRepair:     "repair:select-runtime-accepting-solution-portfolio-mapping",
		claimGraphRepair:       "repair:provide-solution-portfolio-claim-graph",
		relationDiagnosticName: "solution_portfolio",
	}
	return newContract(definition, manifest, adapter)
}

func NewPortfolioComparisonContract(
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) (Contract, error) {
	definition := contractDefinition{
		manifestID:             "haft.portfolio-comparison",
		manifestVersion:        "2.0.0",
		adapterVersion:         "haft-portfolio-comparison-adapter/2.0.0",
		signatureID:            "Haft.PortfolioComparison",
		recordSlotID:           "Haft.PortfolioComparison.ComparisonSlot",
		recordKindID:           projectRecordKindID,
		recordRefID:            projectRecordRefID,
		recordVariant:          (recordcarrier.GenericProjectRecordVariantV1{}).Token(),
		portfolioSlotID:        "Haft.PortfolioComparison.PortfolioSlot",
		concernSlotID:          "Haft.PortfolioComparison.EntityOfConcernSlot",
		claimGraphSlotID:       "Haft.PortfolioComparison.ClaimGraphSlot",
		comparedOptionSlotID:   "Haft.PortfolioComparison.ComparedOptionSlot",
		nonDominatedSlotID:     "Haft.PortfolioComparison.NonDominatedOptionSlot",
		mappingRepair:          "repair:haft-portfolio-comparison-mapping-manifest-v2",
		mappingRegistration:    "portfolio_comparison_mapping_registration",
		registrationRepair:     "repair:select-runtime-accepting-portfolio-comparison-mapping",
		claimGraphRepair:       "repair:provide-portfolio-comparison-claim-graph",
		relationDiagnosticName: "portfolio_comparison",
	}
	return newContract(definition, manifest, adapter)
}

func NewDecisionRecordContract(
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) (Contract, error) {
	definition := contractDefinition{
		manifestID:             "haft.decision-choice-at-concern",
		manifestVersion:        "2.0.0",
		adapterVersion:         "haft-decision-record-adapter/2.0.0",
		signatureID:            "Haft.DecisionChoiceAtConcern",
		recordSlotID:           "Haft.DecisionChoiceAtConcern.DecisionRecordSlot",
		recordKindID:           "Haft.DecisionRecord",
		recordRefID:            "Haft.DecisionRecordRef",
		recordVariant:          (recordcarrier.DecisionRecordVariantV1{}).Token(),
		concernSlotID:          "Haft.DecisionChoiceAtConcern.EntityOfConcernSlot",
		problemSlotID:          "Haft.DecisionChoiceAtConcern.ProblemRecordSlot",
		portfolioSlotID:        "Haft.DecisionChoiceAtConcern.PortfolioRecordSlot",
		optionSlotID:           "Haft.DecisionChoiceAtConcern.OptionSlot",
		chosenOptionSlotID:     "Haft.DecisionChoiceAtConcern.ChosenOptionSlot",
		rejectedOptionSlotID:   "Haft.DecisionChoiceAtConcern.RejectedOptionSlot",
		comparisonSlotID:       "Haft.DecisionChoiceAtConcern.ComparisonRecordSlot",
		claimGraphSlotID:       "Haft.DecisionChoiceAtConcern.ClaimGraphSlot",
		mappingRepair:          "repair:haft-decision-choice-mapping-manifest-v2",
		mappingRegistration:    "decision_choice_mapping_registration",
		registrationRepair:     "repair:select-runtime-accepting-decision-choice-mapping",
		claimGraphRepair:       "repair:recover-bound-decision-choice-result",
		relationDiagnosticName: "decision_choice",
	}
	return newContract(definition, manifest, adapter)
}

func NewSpecSectionContract(
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) (Contract, error) {
	definition := contractDefinition{
		manifestID:             "haft.spec-section-at-concern",
		manifestVersion:        "2.0.0",
		adapterVersion:         "haft-spec-section-adapter/2.0.0",
		signatureID:            "Haft.SpecSectionAtConcern",
		recordSlotID:           "Haft.SpecSectionAtConcern.SpecSectionRecordSlot",
		recordKindID:           specSectionRecordKindID,
		recordRefID:            specSectionRecordRefID,
		recordVariant:          (recordcarrier.SpecSectionRecordVariantV1{}).Token(),
		concernSlotID:          "Haft.SpecSectionAtConcern.EntityOfConcernSlot",
		claimGraphSlotID:       "Haft.SpecSectionAtConcern.ClaimGraphSlot",
		mappingRepair:          "repair:haft-spec-section-mapping-manifest-v2",
		mappingRegistration:    "spec_section_mapping_registration",
		registrationRepair:     "repair:select-runtime-accepting-spec-section-mapping",
		claimGraphRepair:       "repair:provide-spec-section-claim-graph",
		relationDiagnosticName: "spec_section",
	}
	return newContract(definition, manifest, adapter)
}

func newContract(
	definition contractDefinition,
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) (Contract, error) {
	if err := manifest.Verify(); err != nil {
		return Contract{}, fmt.Errorf("record-at-concern mapping manifest: %w", err)
	}
	if err := adapter.Verify(); err != nil {
		return Contract{}, fmt.Errorf("record-at-concern adapter version: %w", err)
	}
	if manifest.ID() != definition.manifestID ||
		manifest.Version() != definition.manifestVersion {
		return Contract{}, fmt.Errorf(
			"record-at-concern mapping manifest = %s@%s, want %s@%s",
			manifest.ID(),
			manifest.Version(),
			definition.manifestID,
			definition.manifestVersion,
		)
	}
	if adapter.String() != definition.adapterVersion {
		return Contract{}, fmt.Errorf(
			"record-at-concern adapter = %s, want %s",
			adapter.String(),
			definition.adapterVersion,
		)
	}
	if _, present := recordCarrierVariant(
		definition.recordVariant,
	); !present {
		return Contract{}, fmt.Errorf(
			"record-at-concern carrier variant %q is unsupported",
			definition.recordVariant,
		)
	}
	if definition.recordKindID == "" || definition.recordRefID == "" {
		return Contract{}, fmt.Errorf(
			"record-at-concern record kind and reference kind are required",
		)
	}
	return Contract{
		definition: definition,
		manifest:   manifest,
		adapter:    adapter,
	}, nil
}

func (contract Contract) valid() bool {
	rebuilt, err := newContract(
		contract.definition,
		contract.manifest,
		contract.adapter,
	)
	return err == nil &&
		rebuilt.definition == contract.definition &&
		rebuilt.manifest == contract.manifest &&
		rebuilt.adapter == contract.adapter
}

func (contract Contract) ManifestRef() recordmapping.MappingManifestRef {
	return contract.manifest
}

func (contract Contract) AdapterVersion() recordmapping.AdapterVersion {
	return contract.adapter
}

func (contract Contract) SignatureID() string {
	return contract.definition.signatureID
}

func recordCarrierVariant(
	token string,
) (recordcarrier.ProjectRecordCarrierVariantV1, bool) {
	variants := map[string]recordcarrier.ProjectRecordCarrierVariantV1{
		(recordcarrier.GenericProjectRecordVariantV1{}).Token(): recordcarrier.GenericProjectRecordVariantV1{},
		(recordcarrier.DecisionRecordVariantV1{}).Token():       recordcarrier.DecisionRecordVariantV1{},
		(recordcarrier.SpecSectionRecordVariantV1{}).Token():    recordcarrier.SpecSectionRecordVariantV1{},
	}
	variant, present := variants[token]
	return variant, present
}
