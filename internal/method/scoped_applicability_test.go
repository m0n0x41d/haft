package method_test

import (
	"strings"
	"testing"

	methodpkg "github.com/m0n0x41d/haft/internal/method"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestScopedMethodPackCatalogAndPullUseExactMixedProfileScope(t *testing.T) {
	matrix := scopedMethodPackTestMatrix(t)
	softwareID := scopedMethodPackTestScopeID(t, "software")
	documentsID := scopedMethodPackTestScopeID(t, "documents")

	softwareCatalog, err := methodpkg.DiscoverCatalogForScope(
		matrix,
		softwareID,
		methodpkg.LifecycleCurrent,
	)
	if err != nil {
		t.Fatalf("DiscoverCatalogForScope software: %v", err)
	}
	report, present := softwareCatalog.Report()
	if !present || report.Summary.Returned == 0 {
		t.Fatalf("required software catalog = %#v, present=%t", report, present)
	}

	documentsCatalog, err := methodpkg.DiscoverCatalogForScope(
		matrix,
		documentsID,
		methodpkg.LifecycleCurrent,
	)
	if err != nil {
		t.Fatalf("DiscoverCatalogForScope documents: %v", err)
	}
	if documentsCatalog.Applicability().Kind() != projectprofile.CapabilityNotApplicable {
		t.Fatalf(
			"documents catalog applicability = %q",
			documentsCatalog.Applicability().Kind(),
		)
	}
	if _, present := documentsCatalog.Report(); present {
		t.Fatal("non-software scope exposed an SWE MethodPack catalog")
	}

	input := methodpkg.PullInput{
		Task:             "change a Go package",
		DeclaredTaskKind: "feature",
		ChangeIntent:     "add_behavior",
	}
	softwarePull, err := methodpkg.PullForScope(matrix, softwareID, input)
	if err != nil {
		t.Fatalf("PullForScope software: %v", err)
	}
	run, present := softwarePull.Run()
	if !present || len(run.Methods) == 0 {
		t.Fatalf("required software pull = %#v, present=%t", run, present)
	}

	documentsPull, err := methodpkg.PullForScope(matrix, documentsID, input)
	if err != nil {
		t.Fatalf("PullForScope documents: %v", err)
	}
	if documentsPull.Applicability().Kind() != projectprofile.CapabilityNotApplicable {
		t.Fatalf(
			"documents pull applicability = %q",
			documentsPull.Applicability().Kind(),
		)
	}
	if _, present := documentsPull.Run(); present {
		t.Fatal("non-software scope manufactured a MethodRun")
	}
}

func TestScopedMethodPackRejectsUnknownExactScope(t *testing.T) {
	matrix := scopedMethodPackTestMatrix(t)
	_, err := methodpkg.PullForScope(
		matrix,
		scopedMethodPackTestScopeID(t, "unknown"),
		methodpkg.PullInput{
			Task:             "change a Go package",
			DeclaredTaskKind: "feature",
			ChangeIntent:     "add_behavior",
		},
	)
	if err == nil || !strings.Contains(err.Error(), "not present") {
		t.Fatalf("unknown-scope error = %v", err)
	}
}

func scopedMethodPackTestMatrix(
	t *testing.T,
) projectprofile.CapabilityApplicabilityMatrix {
	t.Helper()
	software, err := projectprofile.NewSoftwareRealization(
		scopedMethodPackTestScopeID(t, "software"),
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	documents, err := projectprofile.NewNonSoftwareRealization(
		scopedMethodPackTestScopeID(t, "documents"),
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

func scopedMethodPackTestScopeID(t *testing.T, raw string) projectprofile.ScopeID {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID(raw)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	return scopeID
}
