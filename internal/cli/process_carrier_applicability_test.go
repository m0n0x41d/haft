package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestProcessCarrierCheckTreatsNonSoftwareAsNormalNotApplicable(
	t *testing.T,
) {
	t.Parallel()

	matrix := processCarrierMixedCapabilityMatrix(t)
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	missingRoot := filepath.Join(t.TempDir(), "root-that-does-not-exist")

	applicability, err := project.DeriveProjectSpecificationSetApplicability(
		matrix,
		processCarrierScopeID(t, "documents"),
	)
	if err != nil {
		t.Fatalf("DeriveProjectSpecificationSetApplicability: %v", err)
	}
	result, err := processCheckMethodPackCarriersForApplicability(
		missingRoot,
		now.Format(time.RFC3339),
		now.Add(time.Hour).Format(time.RFC3339),
		applicability,
	)
	if err != nil {
		t.Fatalf("processCheckMethodPackCarriersForApplicability: %v", err)
	}
	if result.Status != processCheckStatusNotApplicable {
		t.Fatalf("status = %q, want not_applicable: %#v", result.Status, result)
	}
	if strings.Contains(result.Finding, "missing") ||
		strings.Contains(result.NextAction, "haft init") {
		t.Fatalf("non-software result contains an SWE nag: %#v", result)
	}
	evidence := strings.Join(result.EvidenceRefs, " ")
	for _, want := range []string{
		"profile_scope_id=documents",
		"capability:process_checks=not_applicable",
		"capability:swe_methodpack=not_applicable",
	} {
		if !strings.Contains(evidence, want) {
			t.Fatalf("evidence missing %q: %#v", want, result.EvidenceRefs)
		}
	}
}

func TestProcessCarrierCheckKeepsSoftwareCarrierRequirement(t *testing.T) {
	t.Parallel()

	matrix := processCarrierMixedCapabilityMatrix(t)
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)

	applicability, err := project.DeriveProjectSpecificationSetApplicability(
		matrix,
		processCarrierScopeID(t, "software"),
	)
	if err != nil {
		t.Fatalf("DeriveProjectSpecificationSetApplicability: %v", err)
	}
	result, err := processCheckMethodPackCarriersForApplicability(
		t.TempDir(),
		now.Format(time.RFC3339),
		now.Add(time.Hour).Format(time.RFC3339),
		applicability,
	)
	if err != nil {
		t.Fatalf("processCheckMethodPackCarriersForApplicability: %v", err)
	}
	if result.Status != processCheckStatusDegraded {
		t.Fatalf("status = %q, want degraded: %#v", result.Status, result)
	}
	if !strings.Contains(result.Finding, "missing") {
		t.Fatalf("software result did not retain missing-carrier finding: %#v", result)
	}
	evidence := strings.Join(result.EvidenceRefs, " ")
	if !strings.Contains(evidence, "capability:swe_methodpack=required") {
		t.Fatalf("software result lost applicability evidence: %#v", result.EvidenceRefs)
	}
}

func processCarrierMixedCapabilityMatrix(
	t *testing.T,
) projectprofile.CapabilityApplicabilityMatrix {
	t.Helper()
	software, err := projectprofile.NewSoftwareRealization(
		processCarrierScopeID(t, "software"),
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	documents, err := projectprofile.NewNonSoftwareRealization(
		processCarrierScopeID(t, "documents"),
		projectprofile.NoEntityReference{},
		projectprofile.UnspecifiedKindOrientation{},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	scopes, err := projectprofile.NewScopeSet(
		[]projectprofile.RealizationScope{software, documents},
	)
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}
	return matrix
}

func processCarrierScopeID(t *testing.T, raw string) projectprofile.ScopeID {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID(raw)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	return scopeID
}
