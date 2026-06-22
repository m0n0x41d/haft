package project

import "slices"

type SpecCarrierChangeKind string

const (
	SpecCarrierChangeCarrierOnly         SpecCarrierChangeKind = "carrier_only"
	SpecCarrierChangeSemanticFieldUpdate SpecCarrierChangeKind = "semantic_field_update"
	SpecCarrierChangeRelationshipUpdate  SpecCarrierChangeKind = "relationship_update"
	SpecCarrierChangeMixedUpdate         SpecCarrierChangeKind = "mixed_semantic_and_relationship_update"
	SpecCarrierChangeUnknownHighRisk     SpecCarrierChangeKind = "unknown_high_risk"
)

type SpecCarrierImportPosture string

const (
	SpecCarrierImportPostureNoSemanticMutation SpecCarrierImportPosture = "no_semantic_mutation"
	SpecCarrierImportPostureRecognizedUpdate   SpecCarrierImportPosture = "recognized_update"
	SpecCarrierImportPostureAbstainBlock       SpecCarrierImportPosture = "abstain_block"
)

type specCarrierChangeFieldClass string

const (
	specCarrierChangeFieldHighRisk     specCarrierChangeFieldClass = "high_risk"
	specCarrierChangeFieldScalar       specCarrierChangeFieldClass = "semantic_scalar"
	specCarrierChangeFieldRelationship specCarrierChangeFieldClass = "relationship"
	specCarrierChangeFieldCarrierOnly  specCarrierChangeFieldClass = "carrier_only"
)

type specCarrierChangeFieldRule struct {
	Field   string
	Class   specCarrierChangeFieldClass
	Changed func(before SpecSection, after SpecSection) bool
}

type SpecCarrierChangeReport struct {
	SectionID           string                   `json:"section_id"`
	Kind                SpecCarrierChangeKind    `json:"kind"`
	ImportPosture       SpecCarrierImportPosture `json:"import_posture"`
	ScalarFields        []string                 `json:"scalar_fields,omitempty"`
	RelationshipFields  []string                 `json:"relationship_fields,omitempty"`
	HighRiskFields      []string                 `json:"high_risk_fields,omitempty"`
	CarrierOnlyFields   []string                 `json:"carrier_only_fields,omitempty"`
	SourceOfTruth       string                   `json:"source_of_truth"`
	ApplyBoundary       string                   `json:"apply_boundary"`
	Recoverability      string                   `json:"recoverability"`
	RequiresOperatorAct bool                     `json:"requires_operator_act"`
}

func ClassifySpecSectionCarrierChange(before SpecSection, after SpecSection) SpecCarrierChangeReport {
	report := SpecCarrierChangeReport{
		SectionID:      before.ID,
		SourceOfTruth:  "sql_project_graph",
		ApplyBoundary:  "classification_only_no_sql_mutation",
		Recoverability: "recognized_fields_only",
	}
	if report.SectionID == "" {
		report.SectionID = after.ID
	}

	for _, rule := range specCarrierChangeFieldRegistry() {
		if !rule.Changed(before, after) {
			continue
		}
		report = appendSpecCarrierChangeField(report, rule)
	}

	report.Kind = specCarrierChangeKind(report)
	report.ImportPosture = specCarrierImportPosture(report.Kind)
	report.RequiresOperatorAct = report.ImportPosture != SpecCarrierImportPostureNoSemanticMutation
	return report
}

func specCarrierChangeKind(report SpecCarrierChangeReport) SpecCarrierChangeKind {
	if len(report.HighRiskFields) > 0 {
		return SpecCarrierChangeUnknownHighRisk
	}
	if len(report.ScalarFields) > 0 && len(report.RelationshipFields) > 0 {
		return SpecCarrierChangeMixedUpdate
	}
	if len(report.ScalarFields) > 0 {
		return SpecCarrierChangeSemanticFieldUpdate
	}
	if len(report.RelationshipFields) > 0 {
		return SpecCarrierChangeRelationshipUpdate
	}
	return SpecCarrierChangeCarrierOnly
}

func specCarrierImportPosture(kind SpecCarrierChangeKind) SpecCarrierImportPosture {
	if kind == SpecCarrierChangeCarrierOnly {
		return SpecCarrierImportPostureNoSemanticMutation
	}
	if kind == SpecCarrierChangeUnknownHighRisk {
		return SpecCarrierImportPostureAbstainBlock
	}
	return SpecCarrierImportPostureRecognizedUpdate
}

func appendSpecCarrierChangeField(
	report SpecCarrierChangeReport,
	rule specCarrierChangeFieldRule,
) SpecCarrierChangeReport {
	switch rule.Class {
	case specCarrierChangeFieldHighRisk:
		report.HighRiskFields = append(report.HighRiskFields, rule.Field)
	case specCarrierChangeFieldScalar:
		report.ScalarFields = append(report.ScalarFields, rule.Field)
	case specCarrierChangeFieldRelationship:
		report.RelationshipFields = append(report.RelationshipFields, rule.Field)
	case specCarrierChangeFieldCarrierOnly:
		report.CarrierOnlyFields = append(report.CarrierOnlyFields, rule.Field)
	}
	return report
}

func specCarrierChangeFieldRegistry() []specCarrierChangeFieldRule {
	return []specCarrierChangeFieldRule{
		specCarrierStringFieldRule("id", specCarrierChangeFieldHighRisk, func(section SpecSection) string {
			return section.ID
		}),
		specCarrierStringFieldRule("document_kind", specCarrierChangeFieldHighRisk, func(section SpecSection) string {
			return section.DocumentKind
		}),
		{
			Field: "malformed",
			Class: specCarrierChangeFieldHighRisk,
			Changed: func(before SpecSection, after SpecSection) bool {
				return before.Malformed != after.Malformed
			},
		},
		specCarrierStringFieldRule("spec", specCarrierChangeFieldScalar, func(section SpecSection) string {
			return section.Spec
		}),
		{
			Field: "system_frame",
			Class: specCarrierChangeFieldScalar,
			Changed: func(before SpecSection, after SpecSection) bool {
				return before.SystemFrame != after.SystemFrame
			},
		},
		specCarrierStringFieldRule("kind", specCarrierChangeFieldScalar, func(section SpecSection) string {
			return section.Kind
		}),
		specCarrierStringFieldRule("title", specCarrierChangeFieldScalar, func(section SpecSection) string {
			return section.Title
		}),
		specCarrierStringFieldRule("statement_type", specCarrierChangeFieldScalar, func(section SpecSection) string {
			return section.StatementType
		}),
		specCarrierStringFieldRule("claim_layer", specCarrierChangeFieldScalar, func(section SpecSection) string {
			return section.ClaimLayer
		}),
		specCarrierStringFieldRule("owner", specCarrierChangeFieldScalar, func(section SpecSection) string {
			return section.Owner
		}),
		specCarrierStringFieldRule("status", specCarrierChangeFieldScalar, func(section SpecSection) string {
			return section.Status
		}),
		specCarrierStringFieldRule("valid_until", specCarrierChangeFieldScalar, func(section SpecSection) string {
			return section.ValidUntil
		}),
		specCarrierStringSliceFieldRule("terms", specCarrierChangeFieldRelationship, func(section SpecSection) []string {
			return section.Terms
		}),
		specCarrierStringSliceFieldRule("depends_on", specCarrierChangeFieldRelationship, func(section SpecSection) []string {
			return section.DependsOn
		}),
		specCarrierStringSliceFieldRule("target_refs", specCarrierChangeFieldRelationship, func(section SpecSection) []string {
			return section.TargetRefs
		}),
		{
			Field: "evidence_required",
			Class: specCarrierChangeFieldRelationship,
			Changed: func(before SpecSection, after SpecSection) bool {
				return !slices.Equal(before.EvidenceRequired, after.EvidenceRequired)
			},
		},
		{
			Field: "claims",
			Class: specCarrierChangeFieldRelationship,
			Changed: func(before SpecSection, after SpecSection) bool {
				return !slices.EqualFunc(before.Claims, after.Claims, specClaimEqual)
			},
		},
		specCarrierStringFieldRule("path", specCarrierChangeFieldCarrierOnly, func(section SpecSection) string {
			return section.Path
		}),
		{
			Field: "line",
			Class: specCarrierChangeFieldCarrierOnly,
			Changed: func(before SpecSection, after SpecSection) bool {
				return before.Line != after.Line
			},
		},
	}
}

func specCarrierStringFieldRule(
	field string,
	class specCarrierChangeFieldClass,
	read func(section SpecSection) string,
) specCarrierChangeFieldRule {
	return specCarrierChangeFieldRule{
		Field: field,
		Class: class,
		Changed: func(before SpecSection, after SpecSection) bool {
			return read(before) != read(after)
		},
	}
}

func specCarrierStringSliceFieldRule(
	field string,
	class specCarrierChangeFieldClass,
	read func(section SpecSection) []string,
) specCarrierChangeFieldRule {
	return specCarrierChangeFieldRule{
		Field: field,
		Class: class,
		Changed: func(before SpecSection, after SpecSection) bool {
			return !slices.Equal(read(before), read(after))
		},
	}
}

func specClaimEqual(left SpecClaim, right SpecClaim) bool {
	return left.ID == right.ID &&
		left.Class == right.Class &&
		left.Statement == right.Statement &&
		left.ValidUntil == right.ValidUntil &&
		slices.Equal(left.Scope, right.Scope) &&
		slices.Equal(left.SupportRefs, right.SupportRefs) &&
		slices.Equal(left.EvidenceRefs, right.EvidenceRefs) &&
		slices.Equal(left.GoverningPatternRefs, right.GoverningPatternRefs)
}
