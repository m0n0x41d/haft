package project

import (
	"strings"
	"testing"
)

func TestPlanSoftwareSystemMigrationSeparatesSafeStructureFromPolicy(t *testing.T) {
	plan := PlanSoftwareSystemMigration([]SpecSection{
		{ID: "ES.architecture.001", Kind: "enabling.architecture"},
		{ID: "ES.runtime.001", Kind: "enabling.runtime_policy"},
	})

	if plan.Applicable || plan.UnresolvedCount != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Sections[0].NewKind != "software.selected_structure" {
		t.Fatalf("safe mapping = %+v", plan.Sections[0])
	}
	if !plan.PreservesIDs || !plan.BaselinesRequireReopen || !plan.RequiresSpecSync {
		t.Fatalf("migration safety posture = %+v", plan)
	}
}

func TestRenderSoftwareSystemCarrierOmitsRetiredPolicySections(t *testing.T) {
	legacy := "# Enabling System Spec\n\n" +
		"## ES.architecture.001 Structure\n\n```yaml spec-section\nid: ES.architecture.001\nspec: enabling-system\nkind: enabling.architecture\nstatus: active\n```\n\n" +
		"## ES.runtime.001 Runtime policy\n\n```yaml spec-section\nid: ES.runtime.001\nspec: enabling-system\nkind: enabling.runtime_policy\nstatus: deprecated\n```\n"

	rendered := RenderSoftwareSystemCarrier(legacy)
	if !strings.Contains(rendered, "ES.architecture.001") || !strings.Contains(rendered, "software.selected_structure") {
		t.Fatalf("rendered carrier lost selected structure:\n%s", rendered)
	}
	if strings.Contains(rendered, "ES.runtime.001") || strings.Contains(rendered, "enabling.runtime_policy") {
		t.Fatalf("rendered carrier retained retired policy:\n%s", rendered)
	}
}

func TestPlanSoftwareSystemMigrationAcceptsExplicitSoftwareKinds(t *testing.T) {
	kinds := []string{
		"software.role",
		"software.responsibility_allocation",
		"software.functional_behavior",
		"software.procedural_behavior",
		"software.interfaces",
		"software.constraints",
		"software.selected_structure",
	}
	sections := make([]SpecSection, 0, len(kinds))
	for _, kind := range kinds {
		sections = append(sections, SpecSection{ID: "ES.resolved." + kind, Kind: kind, Status: "active"})
	}

	plan := PlanSoftwareSystemMigration(sections)
	if !plan.Applicable || plan.UnresolvedCount != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	for index, item := range plan.Sections {
		if item.Disposition != SoftwareSystemMigrationSafe || item.NewKind != kinds[index] {
			t.Fatalf("section[%d] = %+v, want preserved explicit software kind %q", index, item, kinds[index])
		}
	}
}

func TestPlanSoftwareSystemMigrationMapsOnlyExactCreatorRolePlaceholder(t *testing.T) {
	plan := PlanSoftwareSystemMigration([]SpecSection{
		{
			ID:         "ES.placeholder.001",
			Kind:       "creator-role",
			Status:     "draft",
			ClaimLayer: "carrier",
		},
		{
			ID:         "ES.creator.001",
			Kind:       "creator-role",
			Status:     "active",
			ClaimLayer: "object",
		},
	})

	if plan.Sections[0].Disposition != SoftwareSystemMigrationSafe || plan.Sections[0].NewKind != "software.role" {
		t.Fatalf("placeholder = %+v", plan.Sections[0])
	}
	if plan.Sections[1].Disposition != SoftwareSystemMigrationUnresolved {
		t.Fatalf("product creator role = %+v, want unresolved", plan.Sections[1])
	}
}

func TestPlanSoftwareSystemMigrationTreatsDeprecatedAndSupersededPolicyAsRetired(t *testing.T) {
	plan := PlanSoftwareSystemMigration([]SpecSection{
		{ID: "ES.runtime.001", Kind: "enabling.runtime_policy", Status: "deprecated"},
		{ID: "ES.agent.001", Kind: "enabling.agent_policy", Status: "superseded"},
	})

	if !plan.Applicable || plan.UnresolvedCount != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	for _, item := range plan.Sections {
		if item.Disposition != SoftwareSystemMigrationRetired || item.NewKind != "" {
			t.Fatalf("retired item = %+v", item)
		}
	}
}

func TestRenderSoftwareSystemCarrierPreservesSectionID(t *testing.T) {
	legacy := "# Enabling System Spec\n\nid: ES.architecture.001\nspec: enabling-system\nkind: enabling.architecture\n"
	rendered := RenderSoftwareSystemCarrier(legacy)

	for _, expected := range []string{"# Software System Spec", "id: ES.architecture.001", "spec: software-system", "kind: software.selected_structure"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("rendered carrier missing %q:\n%s", expected, rendered)
		}
	}
}
