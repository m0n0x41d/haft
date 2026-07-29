package projecttypeenvselectioneffect

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type effectFixture struct {
	identity     ProjectTypeEnvHeadSelectionTransactionIdentity
	dag          ProjectTypeEnvHeadSelectionReferenceDAG
	delta        ProjectTypeEnvActivationDelta
	envelope     ProjectTypeEnvActivationAdmissionEnvelope
	basis        ProjectTypeEnvActivationAdmissionBasis
	manifest     ProjectTypeEnvActivationMaterializationManifest
	activation   CommittedProjectTypeEnvActivation
	result       ProjectTypeEnvHeadSelectionCommittedResult
	receipt      ProjectTypeEnvHeadSelectionReceiptV1
	authorityUse ProjectTypeEnvHeadSelectionAuthorityUseRecord
	work         ProjectTypeEnvHeadCASWorkRecord
	closure      ProjectTypeEnvHeadSelectionClosureV1
}

func TestCanonicalTypesRoundTripAndRejectTrailingBytes(t *testing.T) {
	fixture := newEffectFixture(t, 1)
	cases := []struct {
		name   string
		bytes  []byte
		decode func([]byte) error
	}{
		{
			name:  "transaction_identity",
			bytes: fixture.identity.CanonicalBytes(),
			decode: func(value []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionTransactionIdentity(value)
				return err
			},
		},
		{
			name:  "reference_dag",
			bytes: fixture.dag.CanonicalBytes(),
			decode: func(value []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionReferenceDAG(
					fixture.identity,
					value,
				)
				return err
			},
		},
		{
			name:  "activation_delta",
			bytes: fixture.delta.CanonicalBytes(),
			decode: func(value []byte) error {
				_, err := DecodeProjectTypeEnvActivationDelta(value)
				return err
			},
		},
		{
			name:  "activation_envelope",
			bytes: fixture.envelope.CanonicalBytes(),
			decode: func(value []byte) error {
				_, err := DecodeProjectTypeEnvActivationAdmissionEnvelope(value)
				return err
			},
		},
		{
			name:  "activation_basis",
			bytes: fixture.basis.CanonicalBytes(),
			decode: func(value []byte) error {
				_, err := DecodeProjectTypeEnvActivationAdmissionBasis(value)
				return err
			},
		},
		{
			name:  "activation_manifest",
			bytes: fixture.manifest.CanonicalBytes(),
			decode: func(value []byte) error {
				_, err := DecodeProjectTypeEnvActivationMaterializationManifest(value)
				return err
			},
		},
		{
			name:  "committed_result",
			bytes: fixture.result.CanonicalBytes(),
			decode: func(value []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionCommittedResult(value)
				return err
			},
		},
		{
			name:  "receipt",
			bytes: fixture.receipt.CanonicalBytes(),
			decode: func(value []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionReceiptV1(value)
				return err
			},
		},
		{
			name:  "authority_use",
			bytes: fixture.authorityUse.CanonicalBytes(),
			decode: func(value []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionAuthorityUseRecord(value)
				return err
			},
		},
		{
			name:  "cas_work",
			bytes: fixture.work.CanonicalBytes(),
			decode: func(value []byte) error {
				_, err := DecodeProjectTypeEnvHeadCASWorkRecord(value)
				return err
			},
		},
		{
			name:  "closure",
			bytes: fixture.closure.CanonicalBytes(),
			decode: func(value []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionClosureV1(value)
				return err
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.decode(testCase.bytes); err != nil {
				t.Fatalf("decode canonical value: %v", err)
			}
			trailing := append(append([]byte(nil), testCase.bytes...), 0x01)
			if err := testCase.decode(trailing); err == nil {
				t.Fatal("decode accepted trailing bytes")
			}
			wrongDomain := append([]byte(nil), testCase.bytes...)
			if len(wrongDomain) < 9 {
				t.Fatal("canonical fixture is unexpectedly short")
			}
			wrongDomain[8] ^= 0x01
			if err := testCase.decode(wrongDomain); err == nil {
				t.Fatal("decode accepted wrong domain")
			}
		})
	}
}

func TestCanonicalGettersAreMutationIsolated(t *testing.T) {
	fixture := newEffectFixture(t, 1)
	values := []struct {
		name   string
		bytes  func() []byte
		verify func() error
	}{
		{"identity", fixture.identity.CanonicalBytes, fixture.identity.Verify},
		{
			"dag",
			fixture.dag.CanonicalBytes,
			func() error { return fixture.dag.Verify(fixture.identity) },
		},
		{"delta", fixture.delta.CanonicalBytes, fixture.delta.Verify},
		{"envelope", fixture.envelope.CanonicalBytes, fixture.envelope.Verify},
		{"basis", fixture.basis.CanonicalBytes, fixture.basis.Verify},
		{"manifest", fixture.manifest.CanonicalBytes, fixture.manifest.Verify},
		{"result", fixture.result.CanonicalBytes, fixture.result.Verify},
		{"receipt", fixture.receipt.CanonicalBytes, fixture.receipt.Verify},
		{
			"authority_use",
			fixture.authorityUse.CanonicalBytes,
			fixture.authorityUse.Verify,
		},
		{"cas_work", fixture.work.CanonicalBytes, fixture.work.Verify},
		{"closure", fixture.closure.CanonicalBytes, fixture.closure.Verify},
	}
	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			first := value.bytes()
			first[len(first)-1] ^= 0xff
			if err := value.verify(); err != nil {
				t.Fatalf("mutating getter result changed sealed value: %v", err)
			}
		})
	}
	extensions := fixture.delta.Target().OrderedExtensions()
	extensions[0] = mustExtensionRef(t, 'f')
	if fixture.delta.Target().OrderedExtensions()[0] == extensions[0] {
		t.Fatal("target extension getter leaked mutable slice")
	}
}

func TestTransactionIdentitySensitivityAndStableRefDomainSeparation(t *testing.T) {
	base := transactionIdentityFixture(t)
	baseIdentity := mustIdentity(t, base)
	variants := []ProjectTypeEnvHeadSelectionTransactionIdentityInput{
		withProject(base, mustProject(t, "qnt_deadbeef")),
		withKey(base, mustKey(t, "different-key")),
		withRequest(base, mustTypedDigest(t, '9')),
		withContent(base, mustAuthorityDigest(t, '8')),
		withHeadRevision(base, mustHeadRevision(t, 2)),
		withGraphRevision(base, typedmemory.NewGraphRevision(9)),
	}
	for index, input := range variants {
		identity := mustIdentity(t, input)
		if identity.Ref() == baseIdentity.Ref() {
			t.Fatalf("identity variant %d did not change transaction ref", index)
		}
	}
	dag, err := DeriveProjectTypeEnvHeadSelectionReferenceDAG(baseIdentity)
	if err != nil {
		t.Fatalf("derive reference DAG: %v", err)
	}
	refs := []string{
		dag.AuthorityUseRecordRef().String(),
		dag.WorkRef().String(),
		dag.CASWorkRecordRef().String(),
		dag.GraphIdempotencyKey().String(),
	}
	seen := map[string]struct{}{}
	for _, ref := range refs {
		if _, exists := seen[ref]; exists {
			t.Fatalf("stable refs are not domain separated: %s", ref)
		}
		seen[ref] = struct{}{}
	}
	if reflect.TypeOf(baseIdentity.SuccessorHeadRevision()) ==
		reflect.TypeOf(baseIdentity.CommittedGraphRevision()) {
		t.Fatal("HeadRevision and GraphRevision unexpectedly share a Go type")
	}
}

func TestClosureAuthenticatesLaterDigestsWithoutChangingReceipt(t *testing.T) {
	first := newEffectFixture(t, 1)
	second := newEffectFixture(t, 2)
	if first.receipt.Ref() != second.receipt.Ref() ||
		!bytes.Equal(first.receipt.CanonicalBytes(), second.receipt.CanonicalBytes()) {
		t.Fatal("later use-record content changed the cycle-breaking receipt")
	}
	if first.authorityUse.Ref() != second.authorityUse.Ref() {
		t.Fatal("authority-use occurrence ref changed with record content")
	}
	if first.authorityUse.Digest() == second.authorityUse.Digest() {
		t.Fatal("authority-use record digest ignored verifier edition")
	}
	if first.work.Ref() != second.work.Ref() {
		t.Fatal("CAS Work-record stable ref changed with member content")
	}
	if first.work.Digest() == second.work.Digest() {
		t.Fatal("CAS Work-record digest ignored authority-use digest")
	}
	if first.closure.Ref() == second.closure.Ref() {
		t.Fatal("closure ref ignored member digest changes")
	}
}

func TestCASWorkRecordDoesNotAssertProjectGraphUWorkMembership(t *testing.T) {
	fixture := newEffectFixture(t, 1)
	posture := fixture.work.ProjectGraphUWorkAdmission()
	if posture.String() != projectGraphUWorkNotAssertedP8GV1 {
		t.Fatalf("unexpected project-graph U.Work posture: %q", posture.String())
	}
	workRecordType := reflect.TypeOf(fixture.work)
	for index := 0; index < workRecordType.NumMethod(); index++ {
		method := workRecordType.Method(index)
		if strings.Contains(method.Name, "AdmitAsUWork") ||
			strings.Contains(method.Name, "UWorkMembershipRef") {
			t.Fatalf("CAS Work record exposes project-graph admission: %s", method.Name)
		}
	}
}

func TestCASWorkRecordRejectsExecutionSubjectSubstitution(t *testing.T) {
	fixture := newEffectFixture(t, 1)
	original := fixture.work.Coordinates()
	substitutedRole, err := authority.NewRoleAssignmentRef(
		"role-assignment:substituted-performer",
	)
	mustNoError(t, err)
	substituted, err := newProjectTypeEnvHeadCASWorkCoordinates(
		projectTypeEnvHeadCASWorkCoordinatesRawInput{
			Method:            original.Method(),
			MethodDescription: original.MethodDescription(),
			PerformedBy:       substitutedRole,
			ExecutedWithin:    original.ExecutedWithin(),
			BoundedContext:    original.BoundedContext(),
			WorkInterval:      original.WorkInterval(),
			StatePlane:        original.StatePlane(),
			ResourceLedger:    original.ResourceLedger(),
			Outcome:           original.Outcome(),
			Acceptance:        original.Acceptance(),
			AuditTrace:        original.AuditTrace(),
		},
	)
	mustNoError(t, err)
	_, err = SealProjectTypeEnvHeadCASWorkRecord(
		ProjectTypeEnvHeadCASWorkRecordInput{
			Identity:     fixture.identity,
			ReferenceDAG: fixture.dag,
			Receipt:      fixture.receipt,
			AuthorityUse: fixture.authorityUse,
			Result:       fixture.result,
			Coordinates:  substituted,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "authority subject") {
		t.Fatalf("execution-subject substitution error = %v", err)
	}
}

func TestAuthorityCoordinateVariantsAreClosedAndNonCoercible(t *testing.T) {
	common := mustAuthorityCommon(t)
	strict := mustStrictAuthorityCoordinates(t, common)
	explicit := mustExplicitAuthorityCoordinates(t, common)

	if strict.Kind() != ProjectTypeEnvHeadSelectionAuthorityCoordinatesVerifiedSpeechAct {
		t.Fatalf("strict coordinates kind = %q", strict.Kind().String())
	}
	if _, ok := strict.VerifiedSpeechAct(); !ok {
		t.Fatal("strict coordinates lost strict branch")
	}
	if _, ok := strict.TrustedDedicatedCLIInvocation(); ok {
		t.Fatal("strict coordinates coerced into explicit branch")
	}
	if explicit.Kind() != ProjectTypeEnvHeadSelectionAuthorityCoordinatesTrustedDedicatedCLI {
		t.Fatalf("explicit coordinates kind = %q", explicit.Kind().String())
	}
	if _, ok := explicit.TrustedDedicatedCLIInvocation(); !ok {
		t.Fatal("explicit coordinates lost invocation branch")
	}
	if _, ok := explicit.VerifiedSpeechAct(); ok {
		t.Fatal("explicit coordinates coerced into strict branch")
	}
	if bytes.Equal(
		canonicalAuthorityCoordinates(strict),
		canonicalAuthorityCoordinates(explicit),
	) {
		t.Fatal("authority variants share canonical encoding")
	}
}

func TestClosedResultAlgebra(t *testing.T) {
	fixture := newEffectFixture(t, 1)
	fresh, err := NewFreshlyCommitted(
		fixture.closure,
		CommittedAndObserved{},
	)
	if err != nil {
		t.Fatalf("new freshly committed: %v", err)
	}
	if fresh.Closure().Ref() != fixture.closure.Ref() {
		t.Fatal("fresh result lost closure")
	}
	recovered, err := NewFreshlyCommitted(
		fixture.closure,
		CommitRecoveredByExactClosureReread{},
	)
	if err != nil {
		t.Fatalf("new recovered result: %v", err)
	}
	if _, ok := recovered.Delivery().(CommitRecoveredByExactClosureReread); !ok {
		t.Fatal("recovered delivery posture was not preserved")
	}
	replayed, err := NewReplayedExisting(fixture.closure)
	if err != nil || replayed.Closure().Ref() != fixture.closure.Ref() {
		t.Fatalf("new replayed result: %v", err)
	}
	conflict, err := NewReplayConflict(ReplayConflictInput{
		Key:                    fixture.identity.IdempotencyKey(),
		ExistingRequestDigest:  fixture.identity.RequestDigest(),
		PresentedRequestDigest: mustTypedDigest(t, '9'),
		ExistingContentDigest:  fixture.identity.ContentDigest(),
		PresentedContentDigest: fixture.identity.ContentDigest(),
	})
	if err != nil || conflict.Key() != fixture.identity.IdempotencyKey() {
		t.Fatalf("new replay conflict: %v", err)
	}
	notSelected, err := NewNotSelected(NotSelectedStageDrift())
	if err != nil || notSelected.Reason().String() != notSelectedStageDrift {
		t.Fatalf("new not-selected result: %v", err)
	}
	unknown, err := NewCommitOutcomeUnknown(CommitOutcomeUnknownInput{
		RetryKey:      fixture.identity.IdempotencyKey(),
		RequestDigest: fixture.identity.RequestDigest(),
		ContentDigest: fixture.identity.ContentDigest(),
	})
	if err != nil || unknown.RetryKey() != fixture.identity.IdempotencyKey() {
		t.Fatalf("new commit-unknown result: %v", err)
	}
}

func newEffectFixture(
	t *testing.T,
	verifierEdition uint64,
) effectFixture {
	t.Helper()
	identity := mustIdentity(t, transactionIdentityFixture(t))
	dag, err := DeriveProjectTypeEnvHeadSelectionReferenceDAG(identity)
	mustNoError(t, err)
	target := mustTarget(t)
	proof, err := projecttypeenvselection.ParseNoPriorHeadProofRef(
		"project-typeenv-no-prior-head-proof:" + mustTypedDigest(t, '7').String(),
	)
	mustNoError(t, err)
	predecessor := projecttypeenvselection.NewGenesisStagePredecessor()
	head, err := projecttypeenvselection.ProjectTypeEnvHeadRefForProject(
		identity.Project(),
	)
	mustNoError(t, err)
	delta, err := SealProjectTypeEnvActivationDelta(
		ProjectTypeEnvActivationDeltaInput{
			Identity:              identity,
			ReferenceDAG:          dag,
			Head:                  head,
			Predecessor:           predecessor,
			Target:                target,
			ExpectedGraphRevision: typedmemory.NewGraphRevision(7),
		},
	)
	mustNoError(t, err)
	envelope, err := SealProjectTypeEnvActivationAdmissionEnvelope(delta, dag)
	mustNoError(t, err)
	basis, err := SealProjectTypeEnvActivationAdmissionBasis(delta, envelope)
	mustNoError(t, err)
	graph := mustStorageOwnedGraphCoordinates(t)
	manifest, err := SealProjectTypeEnvActivationMaterializationManifest(
		delta,
		envelope,
		basis,
		graph,
	)
	mustNoError(t, err)
	successorHead, err := projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           identity.Project(),
			SelectedComposite: target.Composite(),
			Revision:          identity.SuccessorHeadRevision(),
		},
	)
	mustNoError(t, err)
	activation, err := SealCommittedProjectTypeEnvActivation(
		CommittedProjectTypeEnvActivationInput{
			Identity:              identity,
			ReferenceDAG:          dag,
			Delta:                 delta,
			Envelope:              envelope,
			Basis:                 basis,
			Manifest:              manifest,
			SuccessorHead:         successorHead,
			MaterializationDigest: mustTypedDigest(t, '6'),
		},
	)
	mustNoError(t, err)
	result, err := SealProjectTypeEnvHeadSelectionCommittedResult(activation)
	mustNoError(t, err)
	receipt, err := SealProjectTypeEnvHeadSelectionReceiptV1(
		ProjectTypeEnvHeadSelectionReceiptInput{
			Identity:     identity,
			ReferenceDAG: dag,
			Authority:    mustAuthorityCoordinates(t),
			Activation:   activation,
			Result:       result,
		},
	)
	mustNoError(t, err)
	verifier, err := NewProjectTypeEnvHeadSelectionVerifierRef(
		"project-typeenv-head-selection-verifier:kernel",
	)
	mustNoError(t, err)
	edition, err := NewProjectTypeEnvHeadSelectionVerifierEdition(verifierEdition)
	mustNoError(t, err)
	authorityUse, err := SealProjectTypeEnvHeadSelectionAuthorityUseRecord(
		ProjectTypeEnvHeadSelectionAuthorityUseRecordInput{
			Identity:        identity,
			ReferenceDAG:    dag,
			Receipt:         receipt,
			Result:          result,
			Verifier:        verifier,
			VerifierEdition: edition,
		},
	)
	mustNoError(t, err)
	work, err := SealProjectTypeEnvHeadCASWorkRecord(
		ProjectTypeEnvHeadCASWorkRecordInput{
			Identity:     identity,
			ReferenceDAG: dag,
			Receipt:      receipt,
			AuthorityUse: authorityUse,
			Result:       result,
			Coordinates:  mustCASWorkCoordinates(t, receipt.AuthorityCoordinates()),
			GenesisProof: proof,
		},
	)
	mustNoError(t, err)
	closure, err := SealProjectTypeEnvHeadSelectionClosureV1(
		ProjectTypeEnvHeadSelectionClosureInput{
			Identity:     identity,
			ReferenceDAG: dag,
			Activation:   activation,
			Result:       result,
			Receipt:      receipt,
			AuthorityUse: authorityUse,
			CASWork:      work,
		},
	)
	mustNoError(t, err)
	return effectFixture{
		identity:     identity,
		dag:          dag,
		delta:        delta,
		envelope:     envelope,
		basis:        basis,
		manifest:     manifest,
		activation:   activation,
		result:       result,
		receipt:      receipt,
		authorityUse: authorityUse,
		work:         work,
		closure:      closure,
	}
}

func transactionIdentityFixture(
	t *testing.T,
) ProjectTypeEnvHeadSelectionTransactionIdentityInput {
	t.Helper()
	requestDigest := mustTypedDigest(t, '1')
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		"project-typeenv-head-selection-request:" + requestDigest.String(),
	)
	mustNoError(t, err)
	return ProjectTypeEnvHeadSelectionTransactionIdentityInput{
		Project:                mustProject(t, "qnt_0123abcd"),
		IdempotencyKey:         mustKey(t, "selection-key"),
		RequestRef:             requestRef,
		RequestDigest:          requestDigest,
		ContentDigest:          mustExecutionSubject(t).AuthorizationContentDigest(),
		SuccessorHeadRevision:  mustHeadRevision(t, 1),
		CommittedGraphRevision: typedmemory.NewGraphRevision(8),
	}
}

func mustIdentity(
	t *testing.T,
	input ProjectTypeEnvHeadSelectionTransactionIdentityInput,
) ProjectTypeEnvHeadSelectionTransactionIdentity {
	t.Helper()
	value, err := SealProjectTypeEnvHeadSelectionTransactionIdentity(input)
	mustNoError(t, err)
	return value
}

func mustTarget(t *testing.T) ProjectTypeEnvHeadSelectionTarget {
	t.Helper()
	base, err := typedmemory.ParseTypeEnvRef(
		"typeenv:" + mustTypedDigest(t, '3').String(),
	)
	mustNoError(t, err)
	runtimeBasis, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(
		"runtime-evaluation-basis:" + mustTypedDigest(t, '4').String(),
	)
	mustNoError(t, err)
	composite, err := typedmemory.ParseTypeEnvRef(
		"typeenv:" + mustTypedDigest(t, '5').String(),
	)
	mustNoError(t, err)
	stage, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(
		"project-typeenv-stage:" + mustTypedDigest(t, '6').String(),
	)
	mustNoError(t, err)
	value, err := NewProjectTypeEnvHeadSelectionTarget(
		ProjectTypeEnvHeadSelectionTargetInput{
			Base:              base,
			OrderedExtensions: []typedmemory.TypeEnvExtensionRef{mustExtensionRef(t, 'a')},
			RuntimeBasis:      runtimeBasis,
			Composite:         composite,
			Stage:             stage,
		},
	)
	mustNoError(t, err)
	return value
}

func mustAuthorityCoordinates(
	t *testing.T,
) ProjectTypeEnvHeadSelectionAuthorityCoordinates {
	t.Helper()
	return mustStrictAuthorityCoordinates(
		t,
		mustAuthorityCommon(t),
	)
}

func mustAuthorityCommon(
	t *testing.T,
) ProjectTypeEnvHeadSelectionAuthorityCommonInput {
	t.Helper()
	subject := mustExecutionSubject(t)
	policyDigest := mustAuthorityDigest(t, 'e')
	policyRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionModePolicyRef(
			"project-typeenv-head-selection-mode-policy:" + policyDigest.String(),
		)
	mustNoError(t, err)
	configDigest := mustAuthorityDigest(t, 'f')
	configRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionConfigAuthorityBasisRef(
			"project-typeenv-head-selection-config-authority-basis:" +
				configDigest.String(),
		)
	mustNoError(t, err)
	return ProjectTypeEnvHeadSelectionAuthorityCommonInput{
		ContentRef:        subject.AuthorizationDescriptionRef(),
		ContentDigest:     subject.AuthorizationContentDigest(),
		PolicyRef:         policyRef,
		PolicyDigest:      policyDigest,
		ConfigBasisRef:    configRef,
		ConfigBasisDigest: configDigest,
		ExecutionSubject:  subject,
	}
}

func mustExecutionSubject(
	t *testing.T,
) projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionSubject {
	t.Helper()
	canonical := []byte(
		`{"schema":"haft.project-typeenv.head-selection-permission-subject-role-assignment/v1","project_id":"qnt_0123abcd","holder_system_ref":"system:haft-software-system","holder_kind":"U.System","role_ref":"role:project-governance-substrate","bounded_context_ref":"bounded-context:project-typeenv-head-selection","assignment_from":"2026-07-17T06:00:00Z","assignment_until":"2026-07-17T08:00:00Z","assignment_policy_ref":"project-typeenv-head-selection-execution-role-policy:sha256:85ec4c0e6572579d973bfbebf977a48732dc87bd56a83f74b2fcd2ab21b7b9ed","assignment_policy_digest":"sha256:85ec4c0e6572579d973bfbebf977a48732dc87bd56a83f74b2fcd2ab21b7b9ed","assignment_policy_edition_ref":"policy-edition:project-typeenv-head-selection-execution-role/v1","assignment_policy_selection":"current_for_new_write_at_seal","system_admission_ref":"system-admission:project-typeenv-head-selection:sha256:23896d4e3fa33b63c06f91dbfe8fee51a58f21c61670c434fa693a1dcaf063a4","system_admission_digest":"sha256:23896d4e3fa33b63c06f91dbfe8fee51a58f21c61670c434fa693a1dcaf063a4","role_admission_ref":"role-admission:project-typeenv-head-selection:sha256:d6fd9d5efaca80381943ad51a814545e726b02e2a5ce2d94ffbe5b325a3d343c","role_admission_digest":"sha256:d6fd9d5efaca80381943ad51a814545e726b02e2a5ce2d94ffbe5b325a3d343c","assignment_justification_ref":"role-assignment-justification:project-typeenv-head-selection:sha256:65b6626bc1d5627b08cd81efb72427ee247c6095627da2d7b40c2c011f48389c","assignment_justification_digest":"sha256:65b6626bc1d5627b08cd81efb72427ee247c6095627da2d7b40c2c011f48389c","assignment_provenance_ref":"role-assignment-provenance:project-typeenv-head-selection:sha256:c9e89a354bd1f1e428590de212dd310566817d9734498b216b0ef6e04581df61","assignment_provenance_digest":"sha256:c9e89a354bd1f1e428590de212dd310566817d9734498b216b0ef6e04581df61","authorization_description_kind":"claim_id","authorization_description_ref":"claim:project-typeenv-head-selection:a","authorization_content_digest":"sha256:38fa348bf22f9fcf3414a0542ab2c83fc51ddd6461df315670891df9143eaeaf"}`,
	)
	subject, err :=
		projecttypeenvselectionauthority.DecodeProjectTypeEnvHeadSelectionPermissionSubject(
			canonical,
		)
	mustNoError(t, err)
	return subject
}

func mustStrictAuthorityCoordinates(
	t *testing.T,
	common ProjectTypeEnvHeadSelectionAuthorityCommonInput,
) ProjectTypeEnvHeadSelectionAuthorityCoordinates {
	t.Helper()
	speechAct, err := authority.NewSpeechActRef("speech-act:selection")
	mustNoError(t, err)
	speechWork, err := authority.NewWorkRef("work:speech-act-selection")
	mustNoError(t, err)
	recordDigest := mustAuthorityDigest(t, 'c')
	recordRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionSpeechActRecordRef(
			"project-typeenv-head-selection-speech-act-record:" +
				recordDigest.String(),
		)
	mustNoError(t, err)
	resolutionDigest := mustAuthorityDigest(t, 'd')
	resolutionRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionAuthorityResolutionRef(
			"project-typeenv-head-selection-authority-resolution:" +
				resolutionDigest.String(),
		)
	mustNoError(t, err)
	permissionDigest := mustAuthorityDigest(t, 'b')
	permissionRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionPermissionRef(
			"project-typeenv-head-selection-permission:" + permissionDigest.String(),
		)
	mustNoError(t, err)
	value, err := NewVerifiedSpeechActAuthorityCoordinates(
		VerifiedSpeechActAuthorityCoordinatesInput{
			Common:                    common,
			SpeechActRef:              speechAct,
			SpeechActWorkRef:          speechWork,
			SpeechActRecordRef:        recordRef,
			SpeechActRecordDigest:     recordDigest,
			PermissionRef:             permissionRef,
			PermissionDigest:          permissionDigest,
			AuthorityResolutionRef:    resolutionRef,
			AuthorityResolutionDigest: resolutionDigest,
		},
	)
	mustNoError(t, err)
	return value
}

func mustExplicitAuthorityCoordinates(
	t *testing.T,
	common ProjectTypeEnvHeadSelectionAuthorityCommonInput,
) ProjectTypeEnvHeadSelectionAuthorityCoordinates {
	t.Helper()
	sourceDigest := mustAuthorityDigest(t, 'a')
	sourceRef, err :=
		projecttypeenvselectionauthority.ParseTrustedDedicatedCLIInvocationSourceRecordRef(
			"project-typeenv-head-selection-trusted-dedicated-cli-invocation:" +
				sourceDigest.String(),
		)
	mustNoError(t, err)
	resolutionDigest := mustAuthorityDigest(t, 'e')
	resolutionRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionAuthorityResolutionRef(
			"project-typeenv-head-selection-authority-resolution:" +
				resolutionDigest.String(),
		)
	mustNoError(t, err)
	value, err := NewTrustedDedicatedCLIAuthorityCoordinates(
		TrustedDedicatedCLIAuthorityCoordinatesInput{
			Common:           common,
			SourceRef:        sourceRef,
			SourceDigest:     sourceDigest,
			ResolutionRef:    resolutionRef,
			ResolutionDigest: resolutionDigest,
		},
	)
	mustNoError(t, err)
	return value
}

func mustStorageOwnedGraphCoordinates(
	t *testing.T,
) ProjectTypeEnvActivationGraphCoordinates {
	t.Helper()
	event, err := projecttypeenvselection.ParseGraphEventRef(
		"typed-memory-event:" + strings.Repeat("7", 64),
	)
	mustNoError(t, err)
	commit, err := projecttypeenvselection.ParseGraphCommitRef(
		"typed-memory-commit:" + strings.Repeat("8", 64),
	)
	mustNoError(t, err)
	value, err := NewProjectTypeEnvActivationGraphCoordinates(
		ProjectTypeEnvActivationGraphCoordinatesInput{
			Event:  event,
			Commit: commit,
		},
	)
	mustNoError(t, err)
	return value
}

func canonicalAuthorityCoordinates(
	value ProjectTypeEnvHeadSelectionAuthorityCoordinates,
) []byte {
	writer := newCanonicalWriter("test.authority-coordinates")
	encodeAuthorityCoordinates(&writer, value)
	return writer.bytes()
}

func mustCASWorkCoordinates(
	t *testing.T,
	authorityCoordinates ProjectTypeEnvHeadSelectionAuthorityCoordinates,
) ProjectTypeEnvHeadCASWorkCoordinates {
	t.Helper()
	method, err := authority.NewMethodRef("method:typeenv-head-cas")
	mustNoError(t, err)
	description, err := authority.NewMethodDescriptionRef(
		"method-description:typeenv-head-cas-v1",
	)
	mustNoError(t, err)
	window, err := authority.NewTimeWindow(
		time.Date(2026, 7, 17, 7, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 17, 7, 0, 1, 0, time.UTC),
	)
	mustNoError(t, err)
	statePlane, err := authority.NewStatePlaneRef("state-plane:typeenv-head")
	mustNoError(t, err)
	ledger, err := authority.NewResourceLedgerRef("resource-ledger:sqlite-tx")
	mustNoError(t, err)
	outcome, err := authority.NewWorkOutcomeRef("outcome:head-selected")
	mustNoError(t, err)
	acceptance, err := authority.NewAcceptancePostureRef("acceptance:exact")
	mustNoError(t, err)
	audit, err := authority.NewAuditTraceRef("audit:head-selection")
	mustNoError(t, err)
	value, err := NewProjectTypeEnvHeadCASWorkCoordinates(
		ProjectTypeEnvHeadCASWorkCoordinatesInput{
			Method:            method,
			MethodDescription: description,
			Authority:         authorityCoordinates,
			WorkInterval:      window,
			StatePlane:        statePlane,
			ResourceLedger:    ledger,
			Outcome:           outcome,
			Acceptance:        acceptance,
			AuditTrace:        audit,
		},
	)
	mustNoError(t, err)
	return value
}

func mustTypedDigest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	value, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat(string(fill), 64),
	)
	mustNoError(t, err)
	return value
}

func mustAuthorityDigest(t *testing.T, fill byte) authority.Digest {
	t.Helper()
	value, err := authority.NewDigest("sha256:" + strings.Repeat(string(fill), 64))
	mustNoError(t, err)
	return value
}

func mustExtensionRef(
	t *testing.T,
	fill byte,
) typedmemory.TypeEnvExtensionRef {
	t.Helper()
	value, err := typedmemory.ParseTypeEnvExtensionRef(
		"typeenv-extension:example.extension@" +
			mustTypedDigest(t, fill).String(),
	)
	mustNoError(t, err)
	return value
}

func mustProject(t *testing.T, raw string) projectidentity.ProjectID {
	t.Helper()
	value, err := projectidentity.ParseProjectID(raw)
	mustNoError(t, err)
	return value
}

func mustKey(
	t *testing.T,
	raw string,
) projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey {
	t.Helper()
	value, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		raw,
	)
	mustNoError(t, err)
	return value
}

func mustHeadRevision(
	t *testing.T,
	value uint64,
) projecttypeenvselection.HeadRevision {
	t.Helper()
	revision, err := projecttypeenvselection.NewHeadRevision(value)
	mustNoError(t, err)
	return revision
}

func withProject(
	input ProjectTypeEnvHeadSelectionTransactionIdentityInput,
	value projectidentity.ProjectID,
) ProjectTypeEnvHeadSelectionTransactionIdentityInput {
	input.Project = value
	return input
}

func withKey(
	input ProjectTypeEnvHeadSelectionTransactionIdentityInput,
	value projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey,
) ProjectTypeEnvHeadSelectionTransactionIdentityInput {
	input.IdempotencyKey = value
	return input
}

func withRequest(
	input ProjectTypeEnvHeadSelectionTransactionIdentityInput,
	digest typedmemory.SHA256Digest,
) ProjectTypeEnvHeadSelectionTransactionIdentityInput {
	ref, _ := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		"project-typeenv-head-selection-request:" + digest.String(),
	)
	input.RequestRef = ref
	input.RequestDigest = digest
	return input
}

func withContent(
	input ProjectTypeEnvHeadSelectionTransactionIdentityInput,
	digest authority.Digest,
) ProjectTypeEnvHeadSelectionTransactionIdentityInput {
	input.ContentDigest = digest
	return input
}

func withHeadRevision(
	input ProjectTypeEnvHeadSelectionTransactionIdentityInput,
	revision projecttypeenvselection.HeadRevision,
) ProjectTypeEnvHeadSelectionTransactionIdentityInput {
	input.SuccessorHeadRevision = revision
	return input
}

func withGraphRevision(
	input ProjectTypeEnvHeadSelectionTransactionIdentityInput,
	revision typedmemory.GraphRevision,
) ProjectTypeEnvHeadSelectionTransactionIdentityInput {
	input.CommittedGraphRevision = revision
	return input
}

func mustNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
