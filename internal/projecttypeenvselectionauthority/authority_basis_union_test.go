package projecttypeenvselectionauthority

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

func TestExecutionRolePolicyAndAssignmentSupportAreSourceExact(t *testing.T) {
	fixture := buildAuthorityFixture(t)
	policy, err := CurrentProjectTypeEnvHeadSelectionExecutionRolePolicy()
	if err != nil {
		t.Fatalf("CurrentProjectTypeEnvHeadSelectionExecutionRolePolicy: %v", err)
	}
	if policy.HolderSystem().String() != executionRoleHolderSystem ||
		policy.Role().String() != executionRole ||
		policy.Edition().String() != currentExecutionRolePolicy {
		t.Fatal("current execution RolePolicy lost its exact holder, role, or edition")
	}
	decodedPolicy, err := DecodeProjectTypeEnvHeadSelectionExecutionRolePolicy(
		policy.CanonicalJSON(),
	)
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvHeadSelectionExecutionRolePolicy: %v", err)
	}
	if decodedPolicy.Ref() != policy.Ref() ||
		decodedPolicy.Digest() != policy.Digest() {
		t.Fatal("registered execution RolePolicy changed identity on decode")
	}
	for _, required := range []string{
		executionRoleSystemPattern,
		executionRolePattern,
		executionRoleAssignmentPattern,
		executionRoleFPFRevision,
		executionRoleFPFSpecDigest,
		executionRoleSSSectionRef,
		executionRoleSSSectionDigest,
		executionRoleTSSectionRef,
		executionRoleTSSectionDigest,
		executionRoleContractPayloadV1,
	} {
		if !bytes.Contains(policy.CanonicalJSON(), []byte(required)) {
			t.Fatalf("execution RolePolicy omitted source coordinate %q", required)
		}
	}

	subject, err := SealProjectTypeEnvHeadSelectionPermissionSubject(
		fixture.content,
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionPermissionSubject: %v", err)
	}
	refs := []string{
		subject.SystemAdmissionRef().String(),
		subject.RoleAdmissionRef().String(),
		subject.AssignmentJustificationRef().String(),
		subject.AssignmentProvenanceRef().String(),
	}
	for index, ref := range refs {
		if ref == "" {
			t.Fatalf("assignment support ref %d is empty", index)
		}
	}
	digests := []authority.Digest{
		subject.SystemAdmissionDigest(),
		subject.RoleAdmissionDigest(),
		subject.AssignmentJustificationDigest(),
		subject.AssignmentProvenanceDigest(),
	}
	for index, digest := range digests {
		if digest.String() == "" {
			t.Fatalf("assignment support digest %d is empty", index)
		}
	}
	supportMaterials := []struct {
		label     string
		domain    string
		canonical []byte
		digest    authority.Digest
	}{
		{
			label:     "A.1 system admission",
			domain:    executionSystemAdmissionDomain,
			canonical: subject.SystemAdmissionCanonicalJSON(),
			digest:    subject.SystemAdmissionDigest(),
		},
		{
			label:     "A.2 Role admission",
			domain:    executionRoleAdmissionDomain,
			canonical: subject.RoleAdmissionCanonicalJSON(),
			digest:    subject.RoleAdmissionDigest(),
		},
		{
			label:     "A.2.1 assignment justification",
			domain:    executionAssignmentJustificationDomain,
			canonical: subject.AssignmentJustificationCanonicalJSON(),
			digest:    subject.AssignmentJustificationDigest(),
		},
		{
			label:     "assignment provenance",
			domain:    executionAssignmentProvenanceDomain,
			canonical: subject.AssignmentProvenanceCanonicalJSON(),
			digest:    subject.AssignmentProvenanceDigest(),
		},
	}
	for _, material := range supportMaterials {
		if len(material.canonical) == 0 {
			t.Fatalf("%s canonical inspect material is empty", material.label)
		}
		recomputed, err := digestCanonical(material.domain, material.canonical)
		if err != nil {
			t.Fatalf("digest %s canonical inspect material: %v", material.label, err)
		}
		if recomputed != material.digest {
			t.Fatalf("%s canonical inspect material differs from its typed digest", material.label)
		}
		mutated := material.canonical
		mutated[0] ^= 1
		switch material.label {
		case "A.1 system admission":
			if bytes.Equal(mutated, subject.SystemAdmissionCanonicalJSON()) {
				t.Fatal("system admission inspect bytes leaked mutable storage")
			}
		case "A.2 Role admission":
			if bytes.Equal(mutated, subject.RoleAdmissionCanonicalJSON()) {
				t.Fatal("Role admission inspect bytes leaked mutable storage")
			}
		case "A.2.1 assignment justification":
			if bytes.Equal(mutated, subject.AssignmentJustificationCanonicalJSON()) {
				t.Fatal("assignment justification inspect bytes leaked mutable storage")
			}
		default:
			if bytes.Equal(mutated, subject.AssignmentProvenanceCanonicalJSON()) {
				t.Fatal("assignment provenance inspect bytes leaked mutable storage")
			}
		}
	}

	crossSubstituted := bytes.Replace(
		subject.CanonicalJSON(),
		[]byte(
			`"system_admission_digest":"`+
				subject.SystemAdmissionDigest().String()+
				`"`,
		),
		[]byte(
			`"system_admission_digest":"`+
				subject.RoleAdmissionDigest().String()+
				`"`,
		),
		1,
	)
	if _, err := DecodeProjectTypeEnvHeadSelectionPermissionSubject(
		crossSubstituted,
	); err == nil {
		t.Fatal("Permission subject accepted a role-admission digest as system admission")
	}
	forgedPolicy := bytes.Replace(
		subject.CanonicalJSON(),
		[]byte(`"assignment_policy_digest":"`+policy.Digest().String()+`"`),
		[]byte(`"assignment_policy_digest":"`+mustAuthorityDigest(t, "0").String()+`"`),
		1,
	)
	if _, err := DecodeProjectTypeEnvHeadSelectionPermissionSubject(
		forgedPolicy,
	); err == nil {
		t.Fatal("Permission subject accepted a forged execution RolePolicy digest")
	}
	staleEdition := bytes.Replace(
		subject.CanonicalJSON(),
		[]byte(`"assignment_policy_edition_ref":"`+executionRolePolicyEditionV1+`"`),
		[]byte(`"assignment_policy_edition_ref":"policy-edition:project-typeenv-head-selection-execution-role/v0"`),
		1,
	)
	if _, err := DecodeProjectTypeEnvHeadSelectionPermissionSubject(
		staleEdition,
	); err == nil {
		t.Fatal("Permission subject accepted an unregistered historical policy edition")
	}

	expiredWindow, err := authority.NewTimeWindow(
		time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 19, 1, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewTimeWindow(expired source): %v", err)
	}
	expiredContent, err := SealProjectTypeEnvHeadSelectionAuthorizationContent(
		ProjectTypeEnvHeadSelectionAuthorizationContentInput{
			DescriptionRef:   fixture.content.DescriptionRef(),
			Request:          fixture.content.Request(),
			Stage:            fixture.content.Stage(),
			JudgementContext: fixture.content.JudgementContext(),
			ValidityWindow:   expiredWindow,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionAuthorizationContent(expired source): %v", err)
	}
	if _, err := SealProjectTypeEnvHeadSelectionPermissionSubject(
		expiredContent,
	); err == nil || !strings.Contains(err.Error(), "policy_source_not_current_for_new_write") {
		t.Fatalf("new assignment did not fail explicitly on stale source policy: %v", err)
	}
	historicalSubject, err := sealProjectTypeEnvHeadSelectionPermissionSubjectWithPolicy(
		expiredContent,
		policy,
		false,
	)
	if err != nil {
		t.Fatalf("seal registered historical subject: %v", err)
	}
	decodedHistoricalSubject, err := DecodeProjectTypeEnvHeadSelectionPermissionSubject(
		historicalSubject.CanonicalJSON(),
	)
	if err != nil {
		t.Fatalf("decode registered historical subject: %v", err)
	}
	if err := decodedHistoricalSubject.Verify(expiredContent); err != nil {
		t.Fatalf("historical subject Verify reused current new-write gate: %v", err)
	}
	if err := decodedHistoricalSubject.VerifyCurrentForUse(
		expiredContent,
	); err == nil || !strings.Contains(err.Error(), "policy_source_not_current_for_new_write") {
		t.Fatalf("historical subject became usable for a new effect: %v", err)
	}
	sourceVariant := defaultSpeechActSourceVariant(
		t,
		fixture.content,
		fixture.request,
	)
	historicalSpeechAct := mustAuthorityValue(
		t,
		authority.NewSpeechActRef,
		sourceVariant.speechActRef,
	)
	historicalPermissionRef, err := deriveProjectTypeEnvHeadSelectionPermissionRefWithSubject(
		expiredContent,
		historicalSpeechAct,
		historicalSubject,
	)
	if err != nil {
		t.Fatalf("derive registered historical Permission ref: %v", err)
	}
	sourceVariant.institutedObject = historicalPermissionRef.String()
	historicalSource := buildSpeechActSourceVariant(
		t,
		expiredContent,
		fixture.request,
		expiredWindow.From().Add(10*time.Minute),
		sourceVariant,
	)
	historicalRecord, err := sealProjectTypeEnvHeadSelectionPermissionRecordWithSubject(
		expiredContent,
		historicalSource,
		historicalSubject,
	)
	if err != nil {
		t.Fatalf("seal registered historical Permission record: %v", err)
	}
	decodedHistorical, err := DecodeProjectTypeEnvHeadSelectionPermissionRecord(
		expiredContent,
		historicalSource,
		historicalRecord.CanonicalJSON(),
		historicalRecord.Digest(),
	)
	if err != nil {
		t.Fatalf("historical Permission decode reused current new-write gate: %v", err)
	}
	if err := decodedHistorical.Verify(
		expiredContent,
		historicalSource,
	); err != nil {
		t.Fatalf("historical Permission verify reused current new-write gate: %v", err)
	}
	if _, err := SealProjectTypeEnvHeadSelectionPermissionRecord(
		expiredContent,
		historicalSource,
	); err == nil || !strings.Contains(err.Error(), "policy_source_not_current_for_new_write") {
		t.Fatalf("new Permission write bypassed current policy after historical replay: %v", err)
	}
}

func TestExecutionRolePolicyDogfoodPinsMatchCurrentSpecCarriers(t *testing.T) {
	software, err := os.ReadFile("../../.haft/specs/software-system.md")
	if err != nil {
		t.Fatalf("read software-system spec: %v", err)
	}
	target, err := os.ReadFile("../../.haft/specs/target-system.md")
	if err != nil {
		t.Fatalf("read target-system spec: %v", err)
	}
	const manifestPath = "../../.context/haft-v9-p2d-evidence/" +
		"active-spec-section-editions.manifest.json"
	manifest, err := os.ReadFile(manifestPath)
	if os.IsNotExist(err) {
		// .context не отслеживается git, поэтому в свежем чекауте манифеста
		// нет. Пропускаем, как это делает internal/recall/liveeval_test.go со
		// своим корпусом: отсутствие носителя — не регрессия.
		t.Skipf("active spec edition manifest not found at %s — skipping", manifestPath)
	}
	if err != nil {
		t.Fatalf("read active spec edition manifest: %v", err)
	}
	for _, required := range [][]byte{
		[]byte("id: " + executionRoleSSSectionRef),
		[]byte("HaftSoftwareSystem fills the holder slot of the ProjectGovernanceSubstrate role assignment"),
	} {
		if !bytes.Contains(software, required) {
			t.Fatalf("software-system spec lost execution RolePolicy source %q", required)
		}
	}
	for _, required := range [][]byte{
		[]byte("id: " + executionRoleTSSectionRef),
		[]byte("holder of the canonical ProjectGovernanceSubstrate role"),
	} {
		if !bytes.Contains(target, required) {
			t.Fatalf("target-system spec lost execution RolePolicy source %q", required)
		}
	}
	for _, required := range [][]byte{
		[]byte(`"section_id":"` + executionRoleSSSectionRef + `","semantic_hash":"` + strings.TrimPrefix(executionRoleSSSectionDigest, "sha256:") + `"`),
		[]byte(`"section_id":"` + executionRoleTSSectionRef + `","semantic_hash":"` + strings.TrimPrefix(executionRoleTSSectionDigest, "sha256:") + `"`),
	} {
		if !bytes.Contains(manifest, required) {
			t.Fatalf("active spec edition manifest lost execution RolePolicy pin %q", required)
		}
	}
}

func TestTrustedDedicatedCLIIngressAndExplicitResolutionRemainDataOnly(t *testing.T) {
	fixture := buildAuthorityFixture(t)
	config := testAuthorityConfigCarrier(t, "1")
	configBasis := testConfigAuthorityBasis(
		t,
		fixture,
		ProjectTypeEnvHeadSelectionAuthorityModeExplicitHDecide,
		config,
	)
	policy, err := SealExplicitHDecideAuthorityPolicy(
		configBasis,
		fixture.binding,
	)
	if err != nil {
		t.Fatalf("SealExplicitHDecideAuthorityPolicy: %v", err)
	}
	recordedAt := fixture.content.ValidityWindow().From().Add(25 * time.Minute)
	sourceInput := TrustedDedicatedCLIInvocationSourceRecordInput{
		Policy:     policy,
		Request:    fixture.request,
		Content:    fixture.content,
		RecordedAt: recordedAt,
	}
	source, err := SealTrustedDedicatedCLIInvocationSourceRecord(sourceInput)
	if err != nil {
		t.Fatalf("SealTrustedDedicatedCLIInvocationSourceRecord: %v", err)
	}
	otherContext := mustAuthorityValue(
		t,
		authority.NewBoundedContextRef,
		"bounded-context:another-project-slice",
	)
	crossContextContent, err := SealProjectTypeEnvHeadSelectionAuthorizationContent(
		ProjectTypeEnvHeadSelectionAuthorizationContentInput{
			DescriptionRef:   fixture.content.DescriptionRef(),
			Request:          fixture.request,
			Stage:            fixture.stage,
			JudgementContext: otherContext,
			ValidityWindow:   fixture.content.ValidityWindow(),
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionAuthorizationContent(cross context): %v", err)
	}
	if _, err := SealTrustedDedicatedCLIInvocationSourceRecord(
		TrustedDedicatedCLIInvocationSourceRecordInput{
			Policy:     policy,
			Request:    fixture.request,
			Content:    crossContextContent,
			RecordedAt: recordedAt,
		},
	); err == nil || !strings.Contains(err.Error(), "outside the explicit policy project-context binding") {
		t.Fatalf("trusted CLI source admitted a cross-context default request: %v", err)
	}
	coordinates := source.Coordinates()
	checks := []struct {
		ok    bool
		label string
	}{
		{coordinates.Mode() == ProjectTypeEnvHeadSelectionAuthorityModeExplicitHDecide, "mode"},
		{coordinates.Project() == fixture.request.Project(), "project"},
		{coordinates.Action() == fixture.content.Action(), "action"},
		{coordinates.RequestRef() == fixture.request.Ref(), "request ref"},
		{coordinates.RequestDigest() == fixture.request.Ref().Digest(), "request digest"},
		{coordinates.ContentRef() == fixture.content.DescriptionRef(), "content ref"},
		{coordinates.ContentDigest() == fixture.content.Digest(), "content digest"},
		{coordinates.PolicyRef() == policy.Ref(), "policy ref"},
		{coordinates.PolicyDigest() == policy.Digest(), "policy digest"},
		{coordinates.ConfigBasisRef() == configBasis.Ref(), "config basis ref"},
		{coordinates.ConfigBasisDigest() == configBasis.Digest(), "config basis digest"},
		{coordinates.ConfigCarrier() == config, "config carrier"},
		{source.Ref().Digest() == source.Digest(), "source identity"},
	}
	for _, check := range checks {
		if !check.ok {
			t.Fatalf("trusted CLI source %s mismatch", check.label)
		}
	}
	for _, forbidden := range [][]byte{
		[]byte(`"speech_act_ref"`),
		[]byte(`"permission_ref"`),
		[]byte(`"human_work_ref"`),
		[]byte(`"observed_h_decide"`),
	} {
		if bytes.Contains(source.CanonicalJSON(), forbidden) {
			t.Fatalf("trusted CLI source fabricated stronger coordinate %q", forbidden)
		}
	}
	decodedSource, err := DecodeTrustedDedicatedCLIInvocationSourceRecord(
		sourceInput,
		source.CanonicalJSON(),
		source.Digest(),
	)
	if err != nil {
		t.Fatalf("DecodeTrustedDedicatedCLIInvocationSourceRecord: %v", err)
	}
	if decodedSource.Ref() != source.Ref() {
		t.Fatal("decoded trusted CLI source changed identity")
	}
	sourceUnion, err := NewAuthoritySourceFromTrustedDedicatedCLIInvocation(source)
	if err != nil {
		t.Fatalf("NewAuthoritySourceFromTrustedDedicatedCLIInvocation: %v", err)
	}
	if sourceUnion.Kind() != AuthoritySourceTrustedDedicatedCLIInvocation {
		t.Fatal("trusted CLI source union has the wrong discriminator")
	}
	if _, ok := sourceUnion.TrustedDedicatedCLIInvocation(); !ok {
		t.Fatal("trusted CLI source union omitted its exact branch")
	}
	if _, ok := sourceUnion.VerifiedSpeechAct(); ok {
		t.Fatal("trusted CLI source was coercible to a verified SpeechAct")
	}

	resolutionInput := ExplicitPolicyAcceptanceResolutionInput{
		Source:      source,
		EvaluatedAt: recordedAt.Add(time.Minute),
	}
	resolution, err := SealExplicitPolicyAcceptanceResolution(resolutionInput)
	if err != nil {
		t.Fatalf("SealExplicitPolicyAcceptanceResolution: %v", err)
	}
	if resolution.Ref().Digest() != resolution.Digest() ||
		resolution.Digest() == source.Digest() {
		t.Fatal("explicit policy resolution identity collapsed")
	}
	for _, forbidden := range [][]byte{
		[]byte(`"speech_act_ref"`),
		[]byte(`"permission_ref"`),
		[]byte(`"human_work_ref"`),
	} {
		if bytes.Contains(resolution.CanonicalJSON(), forbidden) {
			t.Fatalf("explicit policy resolution fabricated stronger coordinate %q", forbidden)
		}
	}
	decodedResolution, err := DecodeExplicitPolicyAcceptanceResolution(
		resolutionInput,
		resolution.CanonicalJSON(),
		resolution.Digest(),
	)
	if err != nil {
		t.Fatalf("DecodeExplicitPolicyAcceptanceResolution: %v", err)
	}
	if decodedResolution.Ref() != resolution.Ref() {
		t.Fatal("decoded explicit policy resolution changed identity")
	}
	resolutionUnion, err := NewAuthorityResolutionFromExplicitPolicyAcceptance(
		resolution,
	)
	if err != nil {
		t.Fatalf("NewAuthorityResolutionFromExplicitPolicyAcceptance: %v", err)
	}
	if resolutionUnion.Kind() != AuthorityResolutionExplicitPolicyAcceptance {
		t.Fatal("explicit resolution union has the wrong discriminator")
	}
	if _, ok := resolutionUnion.ExplicitPolicyAcceptance(); !ok {
		t.Fatal("explicit resolution union omitted its exact branch")
	}
	if _, ok := resolutionUnion.StrictPermission(); ok {
		t.Fatal("explicit resolution was coercible to a strict Permission resolution")
	}
}

func TestVerifiedSpeechActSourceAndStrictPermissionRemainDistinct(t *testing.T) {
	fixture := buildAuthorityFixture(t)
	source, err := SealVerifiedSpeechActAuthoritySourceRecord(fixture.record)
	if err != nil {
		t.Fatalf("SealVerifiedSpeechActAuthoritySourceRecord: %v", err)
	}
	sourceUnion, err := NewAuthoritySourceFromVerifiedSpeechAct(source)
	if err != nil {
		t.Fatalf("NewAuthoritySourceFromVerifiedSpeechAct: %v", err)
	}
	if sourceUnion.Kind() != AuthoritySourceVerifiedSpeechAct {
		t.Fatal("verified SpeechAct source union has the wrong discriminator")
	}
	if _, ok := sourceUnion.VerifiedSpeechAct(); !ok {
		t.Fatal("verified SpeechAct source union omitted its exact branch")
	}
	if _, ok := sourceUnion.TrustedDedicatedCLIInvocation(); ok {
		t.Fatal("verified SpeechAct source was coercible to trusted CLI ingress")
	}

	permission := fixture.resolution.Permission()
	assignment, assignmentOK := fixture.source.PerformedByRoleAssignment()
	assignmentRef, assignmentRefOK := assignment.Ref()
	assignmentDigest, assignmentDigestOK := assignment.Digest()
	if !assignmentOK || !assignmentRefOK || !assignmentDigestOK {
		t.Fatal("verified source role-assignment coordinates are unavailable")
	}
	if permission.SubjectRoleAssignmentRef() == assignmentRef ||
		permission.SubjectRoleAssignmentDigest() == assignmentDigest {
		t.Fatal("Permission subject collapsed into the human SpeechAct performer")
	}
	subject := permission.Subject()
	if subject.Ref() != permission.SubjectRoleAssignmentRef() ||
		subject.Digest() != permission.SubjectRoleAssignmentDigest() ||
		subject.Project() != fixture.content.Project() ||
		subject.HolderSystemRef().String() != executionRoleHolderSystem ||
		subject.RoleRef().String() != executionRole ||
		subject.BoundedContext() != fixture.content.JudgementContext() {
		t.Fatal("Permission subject does not bind the exact HaftSoftwareSystem assignment")
	}
	subjectWindow := subject.AssignmentWindow()
	contentWindow := fixture.content.ValidityWindow()
	if !subjectWindow.From().Equal(contentWindow.From()) ||
		!subjectWindow.Until().Equal(contentWindow.Until()) {
		t.Fatal("Permission subject assignment window differs from reviewed content validity")
	}
	decodedSubject, err := DecodeProjectTypeEnvHeadSelectionPermissionSubject(
		subject.CanonicalJSON(),
	)
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvHeadSelectionPermissionSubject: %v", err)
	}
	if decodedSubject.Ref() != subject.Ref() ||
		decodedSubject.Digest() != subject.Digest() {
		t.Fatal("decoded Permission subject changed identity")
	}
	tamperedSubject := bytes.Replace(
		subject.CanonicalJSON(),
		[]byte(`"holder_kind":"U.System"`),
		[]byte(`"holder_kind":"U.Role"`),
		1,
	)
	if _, err := DecodeProjectTypeEnvHeadSelectionPermissionSubject(
		tamperedSubject,
	); err == nil {
		t.Fatal("Permission subject decoder accepted another holder kind")
	}
	if permission.Modality() != ProjectTypeEnvHeadSelectionPermissionMay {
		t.Fatal("Permission modality is not the closed MAY variant")
	}
	scope := permission.Scope()
	if scope.Ref().String() == "" ||
		scope.Digest().String() == "" ||
		scope.BoundedContext() != fixture.content.JudgementContext() {
		t.Fatal("Permission ClaimScope is incomplete")
	}
	contextPolicy, contextPolicyOK := fixture.source.ContextPolicy()
	contextPolicyRef, contextPolicyRefOK := contextPolicy.Ref()
	contextPolicyDigest, contextPolicyDigestOK := contextPolicy.Digest()
	if !contextPolicyOK || !contextPolicyRefOK || !contextPolicyDigestOK {
		t.Fatal("verified source context-policy coordinates are unavailable")
	}
	if scope.ContextPolicyRef() != contextPolicyRef ||
		scope.ContextPolicyDigest() != contextPolicyDigest {
		t.Fatal("Permission ClaimScope does not bind the source context policy")
	}
	referents := permission.Referents()
	if len(referents) != 2 {
		t.Fatalf("Permission referent count = %d, want 2", len(referents))
	}
	kinds := map[ProjectTypeEnvHeadSelectionPermissionReferentKind]bool{}
	for _, referent := range referents {
		if referent.Ref() == "" || referent.Digest().String() == "" {
			t.Fatal("Permission contains an incomplete typed referent")
		}
		kinds[referent.Kind()] = true
	}
	if !kinds[ProjectTypeEnvHeadSelectionPermissionReferentAuthorizationContent] ||
		!kinds[ProjectTypeEnvHeadSelectionPermissionReferentSelectionRequest] {
		t.Fatal("Permission omitted a required typed referent")
	}
	if !permission.EffectiveFrom().Before(permission.ValidityUntil()) {
		t.Fatal("Permission validity window is empty")
	}
	for _, required := range [][]byte{
		[]byte(`"instituted_kind":"U.Commitment"`),
		[]byte(`"subject_role_assignment_ref":"`),
		[]byte(`"subject_role_assignment":{`),
		[]byte(`"holder_system_ref":"system:haft-software-system"`),
		[]byte(`"role_ref":"role:project-governance-substrate"`),
		[]byte(`"modality":"MAY"`),
		[]byte(`"claim_scope_ref":"`),
		[]byte(`"referents":[`),
		[]byte(`"speech_act_ref":"`),
		[]byte(`"speech_act_work_ref":"`),
		[]byte(`"effective_from":"`),
		[]byte(`"validity_until":"`),
	} {
		if !bytes.Contains(permission.CanonicalJSON(), required) {
			t.Fatalf("Permission canonical projection omitted %s", required)
		}
	}

	resolution := fixture.resolution
	if resolution.Source().Ref() != source.Ref() ||
		resolution.Permission().Ref() != permission.Ref() {
		t.Fatal("strict resolution omitted its exact source or Permission")
	}
	if resolution.Ref().String() == source.Ref().String() ||
		resolution.Ref().String() == permission.Ref().String() ||
		source.Ref().String() == permission.Ref().String() {
		t.Fatal("SpeechAct source, Permission, and resolution identities collapsed")
	}
	resolutionUnion, err := NewAuthorityResolutionFromStrictPermission(resolution)
	if err != nil {
		t.Fatalf("NewAuthorityResolutionFromStrictPermission: %v", err)
	}
	if resolutionUnion.Kind() != AuthorityResolutionStrictPermission {
		t.Fatal("strict resolution union has the wrong discriminator")
	}
	if _, ok := resolutionUnion.StrictPermission(); !ok {
		t.Fatal("strict resolution union omitted its exact branch")
	}
	if _, ok := resolutionUnion.ExplicitPolicyAcceptance(); ok {
		t.Fatal("strict resolution was coercible to explicit policy acceptance")
	}
}

func TestDurableAuthorityUnionsAndCanonicalDecodersFailClosed(t *testing.T) {
	fixture := buildAuthorityFixture(t)
	if (AuthoritySourceRecord{}).Kind() != 0 ||
		(AuthorityResolutionRecord{}).Kind() != 0 ||
		(AuthoritySourceKind(0)).String() != "" ||
		(AuthorityResolutionKind(0)).String() != "" {
		t.Fatal("zero durable union or discriminator became meaningful")
	}
	if _, ok := (AuthoritySourceRecord{}).TrustedDedicatedCLIInvocation(); ok {
		t.Fatal("zero source union exposed a trusted CLI branch")
	}
	if _, ok := (AuthoritySourceRecord{}).VerifiedSpeechAct(); ok {
		t.Fatal("zero source union exposed a verified SpeechAct branch")
	}
	if _, ok := (AuthorityResolutionRecord{}).ExplicitPolicyAcceptance(); ok {
		t.Fatal("zero resolution union exposed an explicit branch")
	}
	if _, ok := (AuthorityResolutionRecord{}).StrictPermission(); ok {
		t.Fatal("zero resolution union exposed a strict branch")
	}
	if _, err := NewAuthoritySourceFromTrustedDedicatedCLIInvocation(
		TrustedDedicatedCLIInvocationSourceRecord{},
	); err == nil {
		t.Fatal("zero trusted CLI source entered the closed union")
	}
	if _, err := NewAuthoritySourceFromVerifiedSpeechAct(
		VerifiedSpeechActAuthoritySourceRecord{},
	); err == nil {
		t.Fatal("zero verified SpeechAct source entered the closed union")
	}
	if _, err := NewAuthorityResolutionFromExplicitPolicyAcceptance(
		ExplicitPolicyAcceptanceResolution{},
	); err == nil {
		t.Fatal("zero explicit resolution entered the closed union")
	}
	if _, err := NewAuthorityResolutionFromStrictPermission(
		StrictPermissionResolution{},
	); err == nil {
		t.Fatal("zero strict resolution entered the closed union")
	}
	if err := (ProjectTypeEnvHeadSelectionPermissionSubject{}).Verify(
		fixture.content,
	); err == nil {
		t.Fatal("zero Permission subject verified as the system assignment")
	}

	configBasis := testConfigAuthorityBasis(
		t,
		fixture,
		ProjectTypeEnvHeadSelectionAuthorityModeExplicitHDecide,
		testAuthorityConfigCarrier(t, "2"),
	)
	policy, err := SealExplicitHDecideAuthorityPolicy(
		configBasis,
		fixture.binding,
	)
	if err != nil {
		t.Fatalf("SealExplicitHDecideAuthorityPolicy: %v", err)
	}
	recordedAt := fixture.content.ValidityWindow().From().Add(30 * time.Minute)
	sourceInput := TrustedDedicatedCLIInvocationSourceRecordInput{
		Policy:     policy,
		Request:    fixture.request,
		Content:    fixture.content,
		RecordedAt: recordedAt,
	}
	source, err := SealTrustedDedicatedCLIInvocationSourceRecord(sourceInput)
	if err != nil {
		t.Fatalf("SealTrustedDedicatedCLIInvocationSourceRecord: %v", err)
	}
	tamperedSource := append([]byte(nil), source.CanonicalJSON()...)
	tamperedSource[len(tamperedSource)-2] ^= 1
	if _, err := DecodeTrustedDedicatedCLIInvocationSourceRecord(
		sourceInput,
		tamperedSource,
		source.Digest(),
	); err == nil {
		t.Fatal("trusted CLI decoder accepted changed canonical bytes")
	}
	if _, err := SealExplicitPolicyAcceptanceResolution(
		ExplicitPolicyAcceptanceResolutionInput{
			Source:      source,
			EvaluatedAt: recordedAt.Add(-time.Minute),
		},
	); err == nil {
		t.Fatal("explicit resolution admitted evaluation before CLI ingress")
	}
	tamperedPermission := append(
		[]byte(nil),
		fixture.record.PermissionRecord().CanonicalJSON()...,
	)
	tamperedPermission[len(tamperedPermission)-2] ^= 1
	if _, err := DecodeProjectTypeEnvHeadSelectionPermissionRecord(
		fixture.content,
		fixture.source,
		tamperedPermission,
		fixture.record.PermissionRecord().Digest(),
	); err == nil {
		t.Fatal("Permission decoder accepted changed canonical bytes")
	}
}

func TestAuthorityModePoliciesStayConfigBoundAndDoNotMintLiveUse(t *testing.T) {
	fixture := buildAuthorityFixture(t)
	config := testAuthorityConfigCarrier(t, "3")
	explicitBasis := testConfigAuthorityBasis(
		t,
		fixture,
		ProjectTypeEnvHeadSelectionAuthorityModeExplicitHDecide,
		config,
	)
	strictBasis := testConfigAuthorityBasis(
		t,
		fixture,
		ProjectTypeEnvHeadSelectionAuthorityModeStrictCLISpeechAct,
		config,
	)
	if _, err := SealExplicitHDecideAuthorityPolicy(
		strictBasis,
		fixture.binding,
	); err == nil {
		t.Fatal("explicit policy accepted a strict config basis")
	}
	if _, err := SealStrictCLISpeechActAuthorityPolicy(
		explicitBasis,
		fixture.policy,
	); err == nil {
		t.Fatal("strict policy accepted an explicit config basis")
	}
	explicitPolicy, err := SealExplicitHDecideAuthorityPolicy(
		explicitBasis,
		fixture.binding,
	)
	if err != nil {
		t.Fatalf("SealExplicitHDecideAuthorityPolicy: %v", err)
	}
	strictPolicy, err := SealStrictCLISpeechActAuthorityPolicy(
		strictBasis,
		fixture.policy,
	)
	if err != nil {
		t.Fatalf("SealStrictCLISpeechActAuthorityPolicy: %v", err)
	}
	if explicitPolicy.Ref() == strictPolicy.Ref() ||
		explicitPolicy.Digest() == strictPolicy.Digest() {
		t.Fatal("explicit and strict authority policies collapsed")
	}
	explicitRecord, err := NewAuthorityPolicyFromExplicitHDecide(explicitPolicy)
	if err != nil {
		t.Fatalf("NewAuthorityPolicyFromExplicitHDecide: %v", err)
	}
	strictRecord, err := NewAuthorityPolicyFromStrictCLISpeechAct(strictPolicy)
	if err != nil {
		t.Fatalf("NewAuthorityPolicyFromStrictCLISpeechAct: %v", err)
	}
	if explicitRecord.Kind() != AuthorityPolicyExplicitHDecide ||
		strictRecord.Kind() != AuthorityPolicyStrictCLISpeechAct {
		t.Fatal("closed policy record has the wrong discriminator")
	}
	if _, ok := explicitRecord.ExplicitHDecide(); !ok {
		t.Fatal("explicit policy record omitted its exact branch")
	}
	if _, ok := explicitRecord.StrictCLISpeechAct(); ok {
		t.Fatal("explicit policy record was coercible to strict")
	}
	if _, ok := strictRecord.StrictCLISpeechAct(); !ok {
		t.Fatal("strict policy record omitted its exact branch")
	}
	if _, ok := strictRecord.ExplicitHDecide(); ok {
		t.Fatal("strict policy record was coercible to explicit")
	}
	if (ProjectTypeEnvHeadSelectionAuthorityPolicyRecord{}).Kind() != 0 ||
		(AuthorityPolicyKind(0)).String() != "" {
		t.Fatal("zero policy record or discriminator became meaningful")
	}
	if _, err := NewAuthorityPolicyFromExplicitHDecide(
		ExplicitHDecideAuthorityPolicy{},
	); err == nil {
		t.Fatal("zero explicit policy entered the closed policy record")
	}
	if _, err := NewAuthorityPolicyFromStrictCLISpeechAct(
		StrictCLISpeechActAuthorityPolicy{},
	); err == nil {
		t.Fatal("zero strict policy entered the closed policy record")
	}
	for _, forbidden := range [][]byte{
		[]byte(`"live_capability"`),
		[]byte(`"admitted_use"`),
		[]byte(`"capability_token"`),
	} {
		if bytes.Contains(explicitPolicy.CanonicalJSON(), forbidden) ||
			bytes.Contains(strictPolicy.CanonicalJSON(), forbidden) {
			t.Fatalf("durable policy minted a live-use coordinate %q", forbidden)
		}
	}
	tampered := append([]byte(nil), explicitPolicy.CanonicalJSON()...)
	tampered[len(tampered)-2] ^= 1
	if _, err := DecodeExplicitHDecideAuthorityPolicy(
		explicitBasis,
		fixture.binding,
		tampered,
		explicitPolicy.Digest(),
	); err == nil {
		t.Fatal("explicit policy decoder accepted changed canonical bytes")
	}
}

func testAuthorityConfigCarrier(
	t *testing.T,
	nibble string,
) authority.ObservableCarrierBinding {
	t.Helper()
	ref := mustAuthorityValue(
		t,
		authority.NewCarrierRef,
		"carrier:.haft/config.yaml:"+nibble,
	)
	digest := mustAuthorityDigest(t, nibble)
	carrier, err := authority.NewObservableCarrierBinding(ref, digest)
	if err != nil {
		t.Fatalf("NewObservableCarrierBinding(config): %v", err)
	}
	return carrier
}

func testConfigAuthorityBasis(
	t *testing.T,
	fixture authorityFixture,
	mode ProjectTypeEnvHeadSelectionAuthorityMode,
	carrier authority.ObservableCarrierBinding,
) ProjectTypeEnvHeadSelectionConfigAuthorityBasis {
	t.Helper()
	basis, err := SealProjectTypeEnvHeadSelectionConfigAuthorityBasis(
		fixture.request.Project(),
		mode,
		carrier,
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionConfigAuthorityBasis: %v", err)
	}
	return basis
}
