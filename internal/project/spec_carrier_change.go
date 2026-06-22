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

	report.HighRiskFields = append(report.HighRiskFields, specCarrierHighRiskFields(before, after)...)
	report.ScalarFields = append(report.ScalarFields, specCarrierScalarFields(before, after)...)
	report.RelationshipFields = append(report.RelationshipFields, specCarrierRelationshipFields(before, after)...)
	report.CarrierOnlyFields = append(report.CarrierOnlyFields, specCarrierCarrierOnlyFields(before, after)...)

	report.Kind = specCarrierChangeKind(report)
	report.ImportPosture = specCarrierImportPosture(report.Kind)
	report.RequiresOperatorAct = report.ImportPosture != SpecCarrierImportPostureNoSemanticMutation
	return report
}

func specCarrierHighRiskFields(before SpecSection, after SpecSection) []string {
	var fields []string
	fields = appendChangedStringField(fields, "id", before.ID, after.ID)
	fields = appendChangedStringField(fields, "document_kind", before.DocumentKind, after.DocumentKind)
	if before.Malformed != after.Malformed {
		fields = append(fields, "malformed")
	}
	return fields
}

func specCarrierScalarFields(before SpecSection, after SpecSection) []string {
	var fields []string
	fields = appendChangedStringField(fields, "spec", before.Spec, after.Spec)
	fields = appendChangedSystemFrameField(fields, before.SystemFrame, after.SystemFrame)
	fields = appendChangedStringField(fields, "kind", before.Kind, after.Kind)
	fields = appendChangedStringField(fields, "title", before.Title, after.Title)
	fields = appendChangedStringField(fields, "statement_type", before.StatementType, after.StatementType)
	fields = appendChangedStringField(fields, "claim_layer", before.ClaimLayer, after.ClaimLayer)
	fields = appendChangedStringField(fields, "owner", before.Owner, after.Owner)
	fields = appendChangedStringField(fields, "status", before.Status, after.Status)
	fields = appendChangedStringField(fields, "valid_until", before.ValidUntil, after.ValidUntil)
	return fields
}

func specCarrierRelationshipFields(before SpecSection, after SpecSection) []string {
	var fields []string
	fields = appendChangedStringSliceField(fields, "terms", before.Terms, after.Terms)
	fields = appendChangedStringSliceField(fields, "depends_on", before.DependsOn, after.DependsOn)
	fields = appendChangedStringSliceField(fields, "target_refs", before.TargetRefs, after.TargetRefs)
	fields = appendChangedEvidenceField(fields, before.EvidenceRequired, after.EvidenceRequired)
	fields = appendChangedClaimsField(fields, before.Claims, after.Claims)
	return fields
}

func specCarrierCarrierOnlyFields(before SpecSection, after SpecSection) []string {
	var fields []string
	fields = appendChangedStringField(fields, "path", before.Path, after.Path)
	if before.Line != after.Line {
		fields = append(fields, "line")
	}
	return fields
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

func appendChangedStringField(fields []string, field string, before string, after string) []string {
	if before == after {
		return fields
	}
	return append(fields, field)
}

func appendChangedSystemFrameField(fields []string, before SystemReferenceFrame, after SystemReferenceFrame) []string {
	if before == after {
		return fields
	}
	return append(fields, "system_frame")
}

func appendChangedStringSliceField(fields []string, field string, before []string, after []string) []string {
	if slices.Equal(before, after) {
		return fields
	}
	return append(fields, field)
}

func appendChangedEvidenceField(fields []string, before []SpecEvidenceRequirement, after []SpecEvidenceRequirement) []string {
	if slices.Equal(before, after) {
		return fields
	}
	return append(fields, "evidence_required")
}

func appendChangedClaimsField(fields []string, before []SpecClaim, after []SpecClaim) []string {
	if slices.EqualFunc(before, after, specClaimEqual) {
		return fields
	}
	return append(fields, "claims")
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
