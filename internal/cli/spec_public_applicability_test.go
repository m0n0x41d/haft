package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestPublicProjectSpecificationApplicabilityOmitsSoftwareForNonSoftwareScope(
	t *testing.T,
) {
	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
			mustCLIProjectNonSoftwareScope(t, "documents"),
		},
	)
	basis := mustCLICanonicalProfileApplicabilityBasis(t, matrix)
	request := mustExactProjectSpecificationScopeRequest(t, "documents")
	resolution, err := projectSpecificationApplicabilityFromMatrix(
		matrix,
		basis,
		request,
	)
	if err != nil {
		t.Fatalf("projectSpecificationApplicabilityFromMatrix: %v", err)
	}

	response, err := publicProjectSpecificationApplicabilityFrom(
		resolution,
		request,
	)
	if err != nil {
		t.Fatalf("publicProjectSpecificationApplicabilityFrom: %v", err)
	}
	if response.Kind != string(projectSpecificationApplicabilityResolved) {
		t.Fatalf("response kind = %q", response.Kind)
	}
	if response.ScopeID != "documents" {
		t.Fatalf("ScopeID = %q", response.ScopeID)
	}
	if response.Cue != nil {
		t.Fatalf("resolved response emitted cue: %#v", response.Cue)
	}
	if !containsPublicSpecString(
		response.ExcludedDocumentKinds,
		string(project.SpecDocumentKindSoftwareSystem),
	) {
		t.Fatalf(
			"excluded document kinds = %#v, want software-system",
			response.ExcludedDocumentKinds,
		)
	}
	if containsPublicSpecString(
		response.ApplicableDocumentKinds,
		string(project.SpecDocumentKindSoftwareSystem),
	) {
		t.Fatalf(
			"applicable document kinds retained software-system: %#v",
			response.ApplicableDocumentKinds,
		)
	}
	if !containsPublicSpecString(
		response.UnderdeterminedKinds,
		string(project.SpecDocumentKindTargetSystem),
	) {
		t.Fatalf(
			"underdetermined document kinds = %#v, want target-system",
			response.UnderdeterminedKinds,
		)
	}
	targetMember, found := publicSpecMemberByDocumentKind(
		response.Members,
		string(project.SpecDocumentKindTargetSystem),
	)
	if !found ||
		targetMember.MissingBasis != "admitted_target_system_relation" {
		t.Fatalf(
			"target-system member = %#v, found=%t; want exact missing basis",
			targetMember,
			found,
		)
	}
	assertPublicProjectSpecificationBasis(t, response, basis)
}

func TestPublicProjectSpecificationApplicabilityRequiresExactMixedScope(
	t *testing.T,
) {
	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
			mustCLIProjectNonSoftwareScope(t, "documents"),
		},
	)
	basis := mustCLICanonicalProfileApplicabilityBasis(t, matrix)
	request := automaticProjectSpecificationScopeRequest()
	resolution, err := projectSpecificationApplicabilityFromMatrix(
		matrix,
		basis,
		request,
	)
	if err != nil {
		t.Fatalf("projectSpecificationApplicabilityFromMatrix: %v", err)
	}

	response, err := publicProjectSpecificationApplicabilityFrom(
		resolution,
		request,
	)
	if err != nil {
		t.Fatalf("publicProjectSpecificationApplicabilityFrom: %v", err)
	}
	if response.Kind != string(projectSpecificationScopeChoiceRequired) {
		t.Fatalf("response kind = %q", response.Kind)
	}
	if response.ScopeID != "" || len(response.Members) != 0 {
		t.Fatalf("mixed automatic response fabricated selection: %#v", response)
	}
	if response.Cue == nil ||
		response.Cue.Code != string(projectSpecificationScopeChoiceRequired) ||
		response.Cue.RequiredInput != "scope_id" {
		t.Fatalf("scope-selection cue = %#v", response.Cue)
	}
	if strings.Join(response.AvailableScopeIDs, ",") != "documents,software" {
		t.Fatalf("available ScopeIDs = %#v", response.AvailableScopeIDs)
	}
	assertPublicProjectSpecificationBasis(t, response, basis)
}

func TestPublicProjectSpecificationApplicabilityRejectsDifferentScopeRequest(
	t *testing.T,
) {
	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
			mustCLIProjectNonSoftwareScope(t, "documents"),
		},
	)
	basis := mustCLICanonicalProfileApplicabilityBasis(t, matrix)
	originatingRequest := mustExactProjectSpecificationScopeRequest(
		t,
		"documents",
	)
	resolution, err := projectSpecificationApplicabilityFromMatrix(
		matrix,
		basis,
		originatingRequest,
	)
	if err != nil {
		t.Fatalf("projectSpecificationApplicabilityFromMatrix: %v", err)
	}

	_, err = publicProjectSpecificationApplicabilityFrom(
		resolution,
		automaticProjectSpecificationScopeRequest(),
	)
	if err == nil {
		t.Fatal("public applicability accepted a request different from its resolution")
	}
}

func TestPublicProjectSpecificationApplicabilityEmitsOneNeutralMissingProfileCue(
	t *testing.T,
) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		t.Context(),
		fixture.root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatalf("resolveCanonicalProjectSpecificationApplicability: %v", err)
	}

	response, err := publicProjectSpecificationApplicabilityFrom(
		resolution,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatalf("publicProjectSpecificationApplicabilityFrom: %v", err)
	}
	if response.Kind != string(projectSpecificationProfileUnderdetermined) {
		t.Fatalf("response kind = %q", response.Kind)
	}
	if response.Cue == nil ||
		response.Cue.Code != string(projectSpecificationProfileUnderdetermined) {
		t.Fatalf("cue = %#v", response.Cue)
	}
	if response.Basis != nil ||
		response.ScopeID != "" ||
		len(response.AvailableScopeIDs) != 0 {
		t.Fatalf("underdetermined response fabricated canonical basis: %#v", response)
	}

	output := bytes.Buffer{}
	if err := writeProjectSpecificationApplicabilityCue(
		&output,
		"haft spec status",
		response,
	); err != nil {
		t.Fatalf("writeProjectSpecificationApplicabilityCue: %v", err)
	}
	if strings.Count(output.String(), "Profile cue:") != 1 {
		t.Fatalf("cue output = %q, want one cue", output.String())
	}
}

func TestMissingProfileApplicabilityPreservesExactOriginatingScopeRequest(
	t *testing.T,
) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	request := mustExactProjectSpecificationScopeRequest(t, "documents")
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		t.Context(),
		fixture.root,
		request,
	)
	if err != nil {
		t.Fatalf("resolveCanonicalProjectSpecificationApplicability: %v", err)
	}

	response, err := publicProjectSpecificationApplicabilityFrom(
		resolution,
		request,
	)
	if err != nil {
		t.Fatalf("publicProjectSpecificationApplicabilityFrom: %v", err)
	}
	if response.Kind != string(projectSpecificationProfileUnderdetermined) {
		t.Fatalf("response kind = %q", response.Kind)
	}
	if response.Request.Kind != string(projectSpecificationScopeExact) ||
		response.Request.ScopeID != "documents" {
		t.Fatalf("public response lost originating request: %#v", response.Request)
	}
}

func TestPublicSpecJSONKeepsLegacyTopLevelFieldsAndExactProfileBasis(
	t *testing.T,
) {
	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{
			mustCLIProjectSoftwareScope(t, "software"),
		},
	)
	basis := mustCLICanonicalProfileApplicabilityBasis(t, matrix)
	request := automaticProjectSpecificationScopeRequest()
	resolution, err := projectSpecificationApplicabilityFromMatrix(
		matrix,
		basis,
		request,
	)
	if err != nil {
		t.Fatalf("projectSpecificationApplicabilityFromMatrix: %v", err)
	}
	applicability, err := publicProjectSpecificationApplicabilityFrom(
		resolution,
		request,
	)
	if err != nil {
		t.Fatalf("publicProjectSpecificationApplicabilityFrom: %v", err)
	}
	report := project.SpecCheckReport{
		Level:     "L0/L1/L1.5",
		Documents: []project.SpecCheckDocument{},
		Findings:  []project.SpecCheckFinding{},
	}
	checkOutput := bytes.Buffer{}
	if err := writePublicSpecCheckJSON(
		&checkOutput,
		publicSpecCheckResult{
			SpecCheckReport:      &report,
			ProfileApplicability: applicability,
		},
	); err != nil {
		t.Fatalf("writePublicSpecCheckJSON: %v", err)
	}
	checkPayload := decodePublicSpecJSON(t, checkOutput.Bytes())
	if checkPayload["level"] != "L0/L1/L1.5" {
		t.Fatalf("check top-level level = %#v", checkPayload["level"])
	}
	assertPublicSpecJSONBasis(t, checkPayload, basis)

	projection := specflow.SpecLifecycleProjection{
		State:  specflow.LifecycleStateReady,
		Action: specflow.LifecycleActionNone,
		Object: "ProjectSpecificationSet",
		Why:    "current scoped specification projection is ready",
	}
	statusOutput := bytes.Buffer{}
	if err := writePublicSpecLifecycleJSON(
		&statusOutput,
		publicSpecLifecycleResult{
			SpecLifecycleProjection: &projection,
			ProfileApplicability:    applicability,
		},
	); err != nil {
		t.Fatalf("writePublicSpecLifecycleJSON: %v", err)
	}
	statusPayload := decodePublicSpecJSON(t, statusOutput.Bytes())
	if statusPayload["state"] != string(specflow.LifecycleStateReady) {
		t.Fatalf("status top-level state = %#v", statusPayload["state"])
	}
	assertPublicSpecJSONBasis(t, statusPayload, basis)
}

func TestScopedPublicSpecHealthDoesNotReloadExcludedSoftwareCarriers(
	t *testing.T,
) {
	root := setupSpecSyncProject(t)
	applicability := mustCLISpecificationApplicability(
		t,
		false,
		"documents",
	)
	specSet, err := loadProjectSpecificationSetSQLFirstForScope(
		root,
		applicability,
	)
	if err != nil {
		t.Fatalf("loadProjectSpecificationSetSQLFirstForScope: %v", err)
	}
	report := project.SpecCheckReportFromSpecificationSet(specSet)
	report = appendSpecHealthFindingsFromSet(report, specSet, root)
	for _, document := range report.Documents {
		if document.Kind == string(project.SpecDocumentKindSoftwareSystem) {
			t.Fatalf(
				"scoped health reloaded excluded software document: %#v",
				document,
			)
		}
	}
	for _, finding := range report.Findings {
		if finding.Code == project.SpecMigrationRequiredFindingCode ||
			strings.Contains(finding.Path, "software-system") {
			t.Fatalf(
				"scoped health reintroduced software pressure: %#v",
				finding,
			)
		}
	}
}

func TestProjectSpecificationScopeRequestFromFlagIsExplicitAndBounded(
	t *testing.T,
) {
	automatic, err := projectSpecificationScopeRequestFromFlag("  ")
	if err != nil {
		t.Fatalf("automatic scope request: %v", err)
	}
	if automatic.kind != projectSpecificationScopeAutomatic {
		t.Fatalf("automatic kind = %q", automatic.kind)
	}

	exact, err := projectSpecificationScopeRequestFromFlag("documents")
	if err != nil {
		t.Fatalf("exact scope request: %v", err)
	}
	if exact.kind != projectSpecificationScopeExact ||
		exact.scopeID.String() != "documents" {
		t.Fatalf("exact request = %#v", exact)
	}

	if _, err := projectSpecificationScopeRequestFromFlag("bad scope"); err == nil {
		t.Fatal("invalid ScopeID was accepted")
	}
	if _, err := projectSpecificationScopeRequestFromFlag(" documents "); err == nil {
		t.Fatal("non-exact whitespace-padded ScopeID was accepted")
	}
}

func TestPublicSpecCommandsExposeOnlyReadScopeSelection(t *testing.T) {
	for _, command := range []*cobra.Command{
		specCheckCmd,
		specStatusCmd,
		specNextCmd,
	} {
		if command.Flags().Lookup("scope-id") == nil {
			t.Fatalf("%s omitted --scope-id", command.CommandPath())
		}
		for _, forbidden := range []string{
			"declare-profile",
			"write-profile",
			"activate-typeenv",
			"approve",
		} {
			if command.Flags().Lookup(forbidden) != nil {
				t.Fatalf(
					"%s exposed forbidden effect flag --%s",
					command.CommandPath(),
					forbidden,
				)
			}
		}
	}
}

func mustExactProjectSpecificationScopeRequest(
	t *testing.T,
	rawScopeID string,
) projectSpecificationScopeRequest {
	t.Helper()
	request, err := projectSpecificationScopeRequestFromFlag(rawScopeID)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func assertPublicProjectSpecificationBasis(
	t *testing.T,
	response publicProjectSpecificationApplicability,
	basis canonicalProfileApplicabilityBasis,
) {
	t.Helper()
	if response.Basis == nil {
		t.Fatal("public response omitted canonical profile basis")
	}
	if response.Basis.ProjectRoot != basis.projectRoot.String() ||
		response.Basis.AdmissionRecordRef != basis.admissionRecordRef.String() ||
		response.Basis.AdmissionRecordDigest != basis.admissionRecordDigest.String() ||
		response.Basis.ProfilePayloadDigest != basis.payloadDigest.String() ||
		response.Basis.LedgerRevision != basis.ledgerRevision.Value() {
		t.Fatalf("public basis = %#v, want exact canonical basis", response.Basis)
	}
}

func assertPublicSpecJSONBasis(
	t *testing.T,
	payload map[string]any,
	basis canonicalProfileApplicabilityBasis,
) {
	t.Helper()
	rawApplicability, ok := payload["profile_applicability"].(map[string]any)
	if !ok {
		t.Fatalf("profile_applicability = %#v", payload["profile_applicability"])
	}
	rawBasis, ok := rawApplicability["basis"].(map[string]any)
	if !ok {
		t.Fatalf("profile basis = %#v", rawApplicability["basis"])
	}
	if rawBasis["admission_record_ref"] != basis.admissionRecordRef.String() ||
		rawBasis["admission_record_digest"] != basis.admissionRecordDigest.String() ||
		rawBasis["profile_payload_digest"] != basis.payloadDigest.String() ||
		rawBasis["ledger_revision"] != float64(basis.ledgerRevision.Value()) {
		t.Fatalf("JSON profile basis = %#v", rawBasis)
	}
}

func decodePublicSpecJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	payload := map[string]any{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode public spec JSON: %v\n%s", err, string(data))
	}
	return payload
}

func containsPublicSpecString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func publicSpecMemberByDocumentKind(
	members []publicProjectSpecificationMemberApplicability,
	documentKind string,
) (publicProjectSpecificationMemberApplicability, bool) {
	for _, member := range members {
		if member.DocumentKind == documentKind {
			return member, true
		}
	}
	return publicProjectSpecificationMemberApplicability{}, false
}
