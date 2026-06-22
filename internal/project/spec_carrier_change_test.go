package project

import (
	"slices"
	"testing"
)

func TestClassifySpecSectionCarrierChange_CarrierOnly(t *testing.T) {
	before := specCarrierChangeSection()
	after := before
	after.Path = ".haft/specs/enabling-system.md"
	after.Line = 42

	report := ClassifySpecSectionCarrierChange(before, after)

	if report.Kind != SpecCarrierChangeCarrierOnly {
		t.Fatalf("kind = %q, want carrier_only", report.Kind)
	}
	if report.ImportPosture != SpecCarrierImportPostureNoSemanticMutation {
		t.Fatalf("import posture = %q", report.ImportPosture)
	}
	if !slices.Equal(report.CarrierOnlyFields, []string{"path", "line"}) {
		t.Fatalf("carrier-only fields = %#v", report.CarrierOnlyFields)
	}
	if report.RequiresOperatorAct {
		t.Fatal("carrier-only move should not require operator act")
	}
}

func TestClassifySpecSectionCarrierChange_SemanticScalarUpdate(t *testing.T) {
	before := specCarrierChangeSection()
	after := before
	after.Title = "Updated title"
	after.ValidUntil = "2026-09-01"

	report := ClassifySpecSectionCarrierChange(before, after)

	if report.Kind != SpecCarrierChangeSemanticFieldUpdate {
		t.Fatalf("kind = %q, want semantic_field_update", report.Kind)
	}
	if report.ImportPosture != SpecCarrierImportPostureRecognizedUpdate {
		t.Fatalf("import posture = %q", report.ImportPosture)
	}
	if !slices.Equal(report.ScalarFields, []string{"title", "valid_until"}) {
		t.Fatalf("scalar fields = %#v", report.ScalarFields)
	}
	if !report.RequiresOperatorAct {
		t.Fatal("semantic update must require an operator act before apply")
	}
}

func TestClassifySpecSectionCarrierChange_RelationshipUpdate(t *testing.T) {
	before := specCarrierChangeSection()
	after := before
	after.DependsOn = []string{"TS.boundary.001"}
	after.TargetRefs = []string{"api_contract:haft_sync"}
	after.Claims = []SpecClaim{
		{
			ID:          "claim-1",
			Class:       "behavior",
			Statement:   "Sync preserves semantic identity.",
			SupportRefs: []string{"dec-sync"},
		},
	}

	report := ClassifySpecSectionCarrierChange(before, after)

	if report.Kind != SpecCarrierChangeRelationshipUpdate {
		t.Fatalf("kind = %q, want relationship_update", report.Kind)
	}
	for _, field := range []string{"depends_on", "target_refs", "claims"} {
		if !slices.Contains(report.RelationshipFields, field) {
			t.Fatalf("relationship fields missing %q: %#v", field, report.RelationshipFields)
		}
	}
}

func TestClassifySpecSectionCarrierChange_MixedRecognizedUpdate(t *testing.T) {
	before := specCarrierChangeSection()
	after := before
	after.Owner = "platform"
	after.Terms = []string{"carrier", "source-of-truth"}

	report := ClassifySpecSectionCarrierChange(before, after)

	if report.Kind != SpecCarrierChangeMixedUpdate {
		t.Fatalf("kind = %q, want mixed update", report.Kind)
	}
	if report.ImportPosture != SpecCarrierImportPostureRecognizedUpdate {
		t.Fatalf("import posture = %q", report.ImportPosture)
	}
}

func TestClassifySpecSectionCarrierChange_UnknownHighRiskBlocks(t *testing.T) {
	before := specCarrierChangeSection()
	after := before
	after.ID = "TS.other.001"
	after.DocumentKind = "enabling-system"
	after.Malformed = true

	report := ClassifySpecSectionCarrierChange(before, after)

	if report.Kind != SpecCarrierChangeUnknownHighRisk {
		t.Fatalf("kind = %q, want unknown_high_risk", report.Kind)
	}
	if report.ImportPosture != SpecCarrierImportPostureAbstainBlock {
		t.Fatalf("import posture = %q", report.ImportPosture)
	}
	for _, field := range []string{"id", "document_kind", "malformed"} {
		if !slices.Contains(report.HighRiskFields, field) {
			t.Fatalf("high-risk fields missing %q: %#v", field, report.HighRiskFields)
		}
	}
	if !report.RequiresOperatorAct {
		t.Fatal("high-risk change must require operator act")
	}
}

func TestSpecCarrierChangeFieldRegistryNamesRecognizedFields(t *testing.T) {
	byClass := map[specCarrierChangeFieldClass][]string{}
	for _, rule := range specCarrierChangeFieldRegistry() {
		byClass[rule.Class] = append(byClass[rule.Class], rule.Field)
		if rule.Changed == nil {
			t.Fatalf("%s missing changed predicate", rule.Field)
		}
	}

	want := map[specCarrierChangeFieldClass][]string{
		specCarrierChangeFieldHighRisk: []string{
			"id",
			"document_kind",
			"malformed",
		},
		specCarrierChangeFieldScalar: []string{
			"spec",
			"system_frame",
			"kind",
			"title",
			"statement_type",
			"claim_layer",
			"owner",
			"status",
			"valid_until",
		},
		specCarrierChangeFieldRelationship: []string{
			"terms",
			"depends_on",
			"target_refs",
			"evidence_required",
			"claims",
		},
		specCarrierChangeFieldCarrierOnly: []string{
			"path",
			"line",
		},
	}
	for class, fields := range want {
		if !slices.Equal(byClass[class], fields) {
			t.Fatalf("%s fields = %#v, want %#v", class, byClass[class], fields)
		}
	}
}

func specCarrierChangeSection() SpecSection {
	return SpecSection{
		ID:            "TS.sync.001",
		Spec:          "target-system",
		SystemFrame:   SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "acceptance",
		Title:         "Sync back",
		StatementType: "definition",
		ClaimLayer:    "normative",
		Owner:         "haft",
		Status:        "active",
		ValidUntil:    "2026-08-01",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
		Line:          10,
	}
}
