package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestPublicSpecCheckRequiresAndSelectsExactMixedScope(t *testing.T) {
	fixture := newMixedProfiledSpecTestProject(t)
	softwareCarrier := filepath.Join(
		fixture.root,
		".haft",
		"specs",
		"software-system.md",
	)
	if err := os.Remove(softwareCarrier); err != nil {
		t.Fatal(err)
	}

	automatic, automaticExit := runPublicSpecCheckForScope(
		t,
		fixture,
		"",
	)
	if automaticExit != 1 || automatic.SpecCheckReport != nil {
		t.Fatalf(
			"automatic mixed result = %#v, exit=%d",
			automatic,
			automaticExit,
		)
	}
	if automatic.ProfileApplicability.Kind !=
		string(projectSpecificationScopeChoiceRequired) ||
		automatic.ProfileApplicability.Cue == nil ||
		automatic.ProfileApplicability.Cue.RequiredInput != "scope_id" {
		t.Fatalf(
			"automatic mixed applicability = %#v",
			automatic.ProfileApplicability,
		)
	}

	documents, documentsExit := runPublicSpecCheckForScope(
		t,
		fixture,
		"documents-spec-cli",
	)
	software, softwareExit := runPublicSpecCheckForScope(
		t,
		fixture,
		"software-spec-cli",
	)
	if documents.SpecCheckReport == nil || software.SpecCheckReport == nil {
		t.Fatalf(
			"exact mixed scope omitted report: documents=%#v software=%#v",
			documents,
			software,
		)
	}
	if documentsExit != 0 {
		t.Fatalf("documents exit = %d, want clean", documentsExit)
	}
	if publicSpecReportContainsDocumentKind(
		*documents.SpecCheckReport,
		string(project.SpecDocumentKindSoftwareSystem),
	) || containsSoftwareSpecHealth(documents.Findings) {
		t.Fatalf(
			"non-software exact scope retained software pressure: %#v",
			documents.SpecCheckReport,
		)
	}
	if softwareExit != 1 ||
		!containsSoftwareSpecHealth(software.Findings) {
		t.Fatalf(
			"software exact scope suppressed missing software carrier: exit=%d report=%#v",
			softwareExit,
			software.SpecCheckReport,
		)
	}
	if documents.ProfileApplicability.ScopeID != "documents-spec-cli" ||
		software.ProfileApplicability.ScopeID != "software-spec-cli" {
		t.Fatalf(
			"exact scope selection changed: documents=%#v software=%#v",
			documents.ProfileApplicability,
			software.ProfileApplicability,
		)
	}
	assertPublicSpecCheckSharedCanonicalBasis(t, documents, software)
}

func TestPublicSpecStatusRequiresAndSelectsExactMixedScope(t *testing.T) {
	fixture := newMixedProfiledSpecTestProject(t)

	automatic := runPublicSpecStatusForScope(t, fixture, "")
	if automatic.SpecLifecycleProjection != nil ||
		automatic.ProfileApplicability.Kind !=
			string(projectSpecificationScopeChoiceRequired) ||
		automatic.ProfileApplicability.Cue == nil ||
		automatic.ProfileApplicability.Cue.RequiredInput != "scope_id" {
		t.Fatalf("automatic mixed lifecycle = %#v", automatic)
	}

	documents := runPublicSpecStatusForScope(
		t,
		fixture,
		"documents-spec-cli",
	)
	software := runPublicSpecStatusForScope(
		t,
		fixture,
		"software-spec-cli",
	)
	if documents.SpecLifecycleProjection == nil ||
		software.SpecLifecycleProjection == nil {
		t.Fatalf(
			"exact mixed lifecycle omitted projection: documents=%#v software=%#v",
			documents,
			software,
		)
	}
	if documents.ProfileApplicability.ScopeID != "documents-spec-cli" ||
		software.ProfileApplicability.ScopeID != "software-spec-cli" {
		t.Fatalf(
			"exact lifecycle scope selection changed: documents=%#v software=%#v",
			documents.ProfileApplicability,
			software.ProfileApplicability,
		)
	}
	if !containsPublicSpecString(
		documents.ProfileApplicability.ExcludedDocumentKinds,
		string(project.SpecDocumentKindSoftwareSystem),
	) || containsPublicSpecString(
		documents.ProfileApplicability.ApplicableDocumentKinds,
		string(project.SpecDocumentKindSoftwareSystem),
	) {
		t.Fatalf(
			"non-software lifecycle applicability = %#v",
			documents.ProfileApplicability,
		)
	}
	if !containsPublicSpecString(
		software.ProfileApplicability.ApplicableDocumentKinds,
		string(project.SpecDocumentKindSoftwareSystem),
	) {
		t.Fatalf(
			"software lifecycle applicability = %#v",
			software.ProfileApplicability,
		)
	}
	assertPublicSpecLifecycleSharedCanonicalBasis(t, documents, software)
}

func runPublicSpecCheckForScope(
	t *testing.T,
	fixture checkTestProject,
	scopeID string,
) (publicSpecCheckResult, int) {
	t.Helper()
	restoreRoot := enterTestProjectRoot(t, fixture.root)
	defer restoreRoot()

	previousJSON := specCheckJSON
	previousScopeID := specCheckScopeID
	previousExit := specCheckExit
	defer func() {
		specCheckJSON = previousJSON
		specCheckScopeID = previousScopeID
		specCheckExit = previousExit
	}()
	specCheckJSON = true
	specCheckScopeID = scopeID
	exitCode := 0
	specCheckExit = func(code int) {
		exitCode = code
	}

	output := bytes.Buffer{}
	command := &cobra.Command{}
	command.SetOut(&output)
	if err := runSpecCheck(command, nil); err != nil {
		t.Fatal(err)
	}
	result := publicSpecCheckResult{}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode public spec check: %v\n%s", err, output.String())
	}
	return result, exitCode
}

func publicSpecReportContainsDocumentKind(
	report project.SpecCheckReport,
	documentKind string,
) bool {
	for _, document := range report.Documents {
		if document.Kind == documentKind {
			return true
		}
	}
	return false
}

func runPublicSpecStatusForScope(
	t *testing.T,
	fixture checkTestProject,
	scopeID string,
) publicSpecLifecycleResult {
	t.Helper()
	restoreRoot := enterTestProjectRoot(t, fixture.root)
	defer restoreRoot()

	previousJSON := specStatusJSON
	previousScopeID := specStatusScopeID
	defer func() {
		specStatusJSON = previousJSON
		specStatusScopeID = previousScopeID
	}()
	specStatusJSON = true
	specStatusScopeID = scopeID

	output := bytes.Buffer{}
	command := &cobra.Command{}
	command.SetOut(&output)
	if err := runSpecStatus(command, nil); err != nil {
		t.Fatal(err)
	}
	result := publicSpecLifecycleResult{}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode public spec status: %v\n%s", err, output.String())
	}
	return result
}

func assertPublicSpecCheckSharedCanonicalBasis(
	t *testing.T,
	left publicSpecCheckResult,
	right publicSpecCheckResult,
) {
	t.Helper()
	leftBasis := left.ProfileApplicability.Basis
	rightBasis := right.ProfileApplicability.Basis
	if leftBasis == nil || rightBasis == nil {
		t.Fatalf("exact mixed result omitted basis: left=%#v right=%#v", leftBasis, rightBasis)
	}
	if leftBasis.AdmissionRecordRef != rightBasis.AdmissionRecordRef ||
		leftBasis.AdmissionRecordDigest != rightBasis.AdmissionRecordDigest ||
		leftBasis.ProfilePayloadDigest != rightBasis.ProfilePayloadDigest ||
		leftBasis.LedgerRevision != rightBasis.LedgerRevision {
		t.Fatalf(
			"exact scopes did not share one canonical profile basis: left=%#v right=%#v",
			leftBasis,
			rightBasis,
		)
	}
}

func assertPublicSpecLifecycleSharedCanonicalBasis(
	t *testing.T,
	left publicSpecLifecycleResult,
	right publicSpecLifecycleResult,
) {
	t.Helper()
	leftBasis := left.ProfileApplicability.Basis
	rightBasis := right.ProfileApplicability.Basis
	if leftBasis == nil || rightBasis == nil {
		t.Fatalf("exact mixed lifecycle omitted basis: left=%#v right=%#v", leftBasis, rightBasis)
	}
	if leftBasis.AdmissionRecordRef != rightBasis.AdmissionRecordRef ||
		leftBasis.AdmissionRecordDigest != rightBasis.AdmissionRecordDigest ||
		leftBasis.ProfilePayloadDigest != rightBasis.ProfilePayloadDigest ||
		leftBasis.LedgerRevision != rightBasis.LedgerRevision {
		t.Fatalf(
			"exact lifecycle scopes did not share one canonical profile basis: left=%#v right=%#v",
			leftBasis,
			rightBasis,
		)
	}
}
