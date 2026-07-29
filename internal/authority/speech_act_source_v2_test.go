package authority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestVerifiedSpeechActSourceV2CanonicalRoundTripAndV1BasisPreservation(
	t *testing.T,
) {
	basis := testVerifiedAuthorityAct(t).state.source
	beforeV1 := append([]byte{}, basis.state.speechAct.state.canonicalJSON...)
	anchors := testSpeechActSourceV2Anchors(t, basis, "work:communicative:test", "carrier:description:test")

	source, err := NewVerifiedSpeechActSourceV2(basis, anchors)
	if err != nil {
		t.Fatalf("NewVerifiedSpeechActSourceV2: %v", err)
	}
	if !source.Valid() {
		t.Fatal("new SpeechAct source v2 is invalid")
	}
	if !bytes.Equal(beforeV1, basis.state.speechAct.state.canonicalJSON) {
		t.Fatal("v2 construction changed v1 canonical SpeechAct bytes")
	}
	canonicalJSON, canonicalOK := source.CanonicalJSON()
	digest, digestOK := source.Digest()
	if !canonicalOK || !digestOK {
		t.Fatal("v2 source omitted its exact canonical identity")
	}
	decoded, err := DecodeVerifiedSpeechActSourceV2(basis, canonicalJSON, digest)
	if err != nil {
		t.Fatalf("DecodeVerifiedSpeechActSourceV2: %v", err)
	}
	decodedJSON, _ := decoded.CanonicalJSON()
	decodedDigest, _ := decoded.Digest()
	if !bytes.Equal(decodedJSON, canonicalJSON) || decodedDigest != digest {
		t.Fatal("v2 canonical round trip changed bytes or digest")
	}

	projection := speechActSourceV2Projection{}
	if err := json.Unmarshal(canonicalJSON, &projection); err != nil {
		t.Fatalf("decode canonical projection: %v", err)
	}
	assertSpeechActSourceV2Projection(t, basis, projection)
	assertSpeechActSourceV2Accessors(t, source)
}

func TestDecodeRecordedSpeechActSourceV2KeepsV1LoaderReadOnlyCompatible(
	t *testing.T,
) {
	_, verified, recorded := recordSpeechActSourceResolverFixture(t)
	anchors := testSpeechActSourceV2Anchors(
		t,
		verified,
		"work:communicative:recorded-test",
		"carrier:description:recorded-test",
	)
	source, err := NewVerifiedSpeechActSourceV2(verified, anchors)
	if err != nil {
		t.Fatalf("NewVerifiedSpeechActSourceV2: %v", err)
	}
	canonicalJSON, _ := source.CanonicalJSON()
	digest, _ := source.Digest()

	decoded, err := DecodeRecordedSpeechActSourceV2(recorded, canonicalJSON, digest)
	if err != nil {
		t.Fatalf("DecodeRecordedSpeechActSourceV2: %v", err)
	}
	decodedJSON, _ := decoded.CanonicalJSON()
	if !bytes.Equal(decodedJSON, canonicalJSON) {
		t.Fatal("recorded-v1 compatibility decode changed v2 canonical bytes")
	}
}

func TestVerifiedSpeechActSourceV2RejectsReferenceCollapse(t *testing.T) {
	basis := testVerifiedAuthorityAct(t).state.source
	act := basis.state.speechAct.state
	description := act.reviewSubjectRef.String()
	speechAct := act.ref.String()
	capture := act.captureCarrierRef.String()
	tests := []struct {
		name        string
		work        string
		description string
		carrier     string
	}{
		{name: "WorkRef is SpeechActRef", work: speechAct, description: description, carrier: "carrier:content:a"},
		{name: "DescriptionRef is SpeechActRef", work: "work:distinct:a", description: speechAct, carrier: "carrier:content:b"},
		{name: "DescriptionRef is WorkRef", work: description, description: description, carrier: "carrier:content:c"},
		{name: "WorkRef is terminal capture", work: capture, description: description, carrier: "carrier:content:d"},
		{name: "DescriptionRef is terminal capture", work: "work:distinct:e", description: capture, carrier: "carrier:content:e"},
		{name: "content CarrierRef is SpeechActRef", work: "work:distinct:f", description: description, carrier: speechAct},
		{name: "content CarrierRef is WorkRef", work: "work:distinct:g", description: description, carrier: "work:distinct:g"},
		{name: "content CarrierRef is DescriptionRef", work: "work:distinct:h", description: description, carrier: description},
		{name: "content CarrierRef is terminal capture", work: "work:distinct:i", description: description, carrier: capture},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			anchors := testSpeechActSourceV2Anchors(
				t,
				basis,
				test.work,
				test.carrier,
				withSpeechActSourceV2Description(test.description),
			)
			if _, err := NewVerifiedSpeechActSourceV2(basis, anchors); err == nil {
				t.Fatal("v2 source accepted collapsed strong-reference roles")
			}
		})
	}
}

func TestVerifiedSpeechActSourceV2RejectsV1BasisWithCollapsedSpeechAndCaptureRefs(
	t *testing.T,
) {
	basis := testSpeechActSourceWithCollapsedSpeechAndCaptureRefs(t)
	if !basis.Valid() {
		t.Fatal("v1 fixture is not a package-verified source")
	}
	act := basis.state.speechAct.state
	if act.ref.String() != act.captureCarrierRef.String() {
		t.Fatal("fixture did not exercise the v1 reference-role collapse")
	}
	anchors := testSpeechActSourceV2Anchors(
		t,
		basis,
		"work:communicative:collapsed-basis",
		"carrier:description:collapsed-basis",
	)
	if _, err := NewVerifiedSpeechActSourceV2(basis, anchors); err == nil {
		t.Fatal("v2 source accepted a SpeechActRef collapsed with terminal CaptureRef")
	}
}

func TestSpeechActSourceV2DescriptionRefIsClosedClaimOrEpisteme(t *testing.T) {
	claim, err := NewClaimIDDescriptionRef("claim:head-selection:test")
	if err != nil || claim.Kind() != DescriptionRefClaimID {
		t.Fatalf("claim DescriptionRef = %#v, %v", claim, err)
	}
	episteme, err := NewEpistemeDescriptionRef("episteme:head-selection:test")
	if err != nil || episteme.Kind() != DescriptionRefEpisteme {
		t.Fatalf("episteme DescriptionRef = %#v, %v", episteme, err)
	}
	if _, err := newDescriptionRef("carrier", "carrier:not-a-description"); err == nil {
		t.Fatal("DescriptionRef admitted a carrier variant")
	}
}

func TestDecodeVerifiedSpeechActSourceV2RejectsNonCanonicalOrForeignMaterial(
	t *testing.T,
) {
	basis := testVerifiedAuthorityAct(t).state.source
	anchors := testSpeechActSourceV2Anchors(
		t,
		basis,
		"work:communicative:decode-test",
		"carrier:description:decode-test",
	)
	source, err := NewVerifiedSpeechActSourceV2(basis, anchors)
	if err != nil {
		t.Fatalf("NewVerifiedSpeechActSourceV2: %v", err)
	}
	canonicalJSON, _ := source.CanonicalJSON()
	digest, _ := source.Digest()
	projection := speechActSourceV2Projection{}
	if err := json.Unmarshal(canonicalJSON, &projection); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	wrongMethod := projection
	wrongMethod.MethodRef = "method:foreign"
	wrongMethodJSON, _ := json.Marshal(wrongMethod)
	unknownDescription := projection
	unknownDescription.Description.Kind = "carrier"
	unknownDescriptionJSON, _ := json.Marshal(unknownDescription)
	wrongOutcome := projection
	wrongOutcome.OutcomeRef = "outcome:foreign"
	wrongOutcomeJSON, _ := json.Marshal(wrongOutcome)
	wrongDescriptionDigest := projection
	wrongDescriptionDigest.Description.Digest = testAuthorityDigest("f")
	wrongDescriptionDigestJSON, _ := json.Marshal(wrongDescriptionDigest)
	withUnknownField := append([]byte{}, canonicalJSON[:len(canonicalJSON)-1]...)
	withUnknownField = append(withUnknownField, []byte(`,"unknown":true}`)...)
	withWhitespace := append([]byte(" "), canonicalJSON...)
	withTrailingJSON := append(append([]byte{}, canonicalJSON...), []byte(` {}`)...)
	foreignDigest := mustParse(t, NewDigest, testAuthorityDigest("e"))

	tests := []struct {
		name   string
		body   []byte
		digest Digest
	}{
		{name: "foreign method", body: wrongMethodJSON, digest: digest},
		{name: "unknown DescriptionRef variant", body: unknownDescriptionJSON, digest: digest},
		{name: "foreign outcome", body: wrongOutcomeJSON, digest: digest},
		{name: "foreign description digest", body: wrongDescriptionDigestJSON, digest: digest},
		{name: "unknown field", body: withUnknownField, digest: digest},
		{name: "non-canonical whitespace", body: withWhitespace, digest: digest},
		{name: "trailing JSON", body: withTrailingJSON, digest: digest},
		{name: "foreign digest", body: canonicalJSON, digest: foreignDigest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decoded, err := DecodeVerifiedSpeechActSourceV2(basis, test.body, test.digest)
			if err == nil || decoded.Valid() {
				t.Fatal("strict v2 decoder accepted foreign or non-canonical material")
			}
		})
	}
}

func TestSpeechActSourceV2RequiresRelianceBearingAnchors(t *testing.T) {
	basis := testVerifiedAuthorityAct(t).state.source
	description := mustClaimDescriptionRef(t, basis.state.speechAct.state.reviewSubjectRef.String())
	work := mustParse(t, NewWorkRef, "work:communicative:missing-anchor-test")
	resource := mustParse(t, NewResourceLedgerRef, "resource-ledger:missing-anchor-test")
	acceptance := mustParse(t, NewAcceptancePostureRef, "acceptance:missing-anchor-test")
	builder := NewSpeechActSourceV2AnchorsBuilder(work, description)
	builder = builder.WithResourceLedger(resource)
	builder = builder.WithAcceptancePosture(acceptance)

	if _, err := builder.Build(); err == nil || !strings.Contains(err.Error(), "audit-trace") {
		t.Fatalf("missing audit-trace error = %v", err)
	}
	audit := mustParse(t, NewAuditTraceRef, "audit:missing-anchor-test")
	builder = builder.WithAuditTrace(audit)
	if _, err := builder.Build(); err == nil || !strings.Contains(err.Error(), "description carrier") {
		t.Fatalf("missing content-carrier error = %v", err)
	}
}

func TestSpeechActSourceV2EnforcesReviewedCollectionAndByteLimits(t *testing.T) {
	basis := testVerifiedAuthorityAct(t).state.source
	description := mustClaimDescriptionRef(t, basis.state.speechAct.state.reviewSubjectRef.String())
	work := mustParse(t, NewWorkRef, "work:communicative:limit-test")
	resource := mustParse(t, NewResourceLedgerRef, "resource-ledger:limit-test")
	acceptance := mustParse(t, NewAcceptancePostureRef, "acceptance:limit-test")
	baseBuilder := NewSpeechActSourceV2AnchorsBuilder(work, description)
	baseBuilder = baseBuilder.WithResourceLedger(resource)
	baseBuilder = baseBuilder.WithAcceptancePosture(acceptance)

	boundaryBuilder := addSpeechActSourceV2AuditFixtures(
		t,
		baseBuilder,
		speechActSourceV2MaxAuditTraceRefs,
		0,
	)
	boundaryBuilder = addSpeechActSourceV2CarrierFixtures(
		t,
		boundaryBuilder,
		speechActSourceV2MaxDescriptionCarriers,
		0,
	)
	boundary, err := boundaryBuilder.Build()
	if err != nil {
		t.Fatalf("reviewed collection boundary rejected: %v", err)
	}
	boundarySource, err := NewVerifiedSpeechActSourceV2(basis, boundary)
	if err != nil {
		t.Fatalf("reviewed v2 source boundary rejected: %v", err)
	}

	overAuditBuilder := addSpeechActSourceV2AuditFixtures(
		t,
		baseBuilder,
		speechActSourceV2MaxAuditTraceRefs+1,
		0,
	)
	overAuditBuilder = addSpeechActSourceV2CarrierFixtures(t, overAuditBuilder, 1, 0)
	if _, err := overAuditBuilder.Build(); err == nil || !strings.Contains(err.Error(), "1..64") {
		t.Fatalf("over-limit audit refs error = %v", err)
	}

	overCarrierBuilder := addSpeechActSourceV2AuditFixtures(t, baseBuilder, 1, 0)
	overCarrierBuilder = addSpeechActSourceV2CarrierFixtures(
		t,
		overCarrierBuilder,
		speechActSourceV2MaxDescriptionCarriers+1,
		0,
	)
	if _, err := overCarrierBuilder.Build(); err == nil || !strings.Contains(err.Error(), "1..16") {
		t.Fatalf("over-limit description carriers error = %v", err)
	}

	digest := mustParse(t, NewDigest, testAuthorityDigest("d"))
	oversized := bytes.Repeat([]byte{' '}, speechActSourceV2MaxCanonicalBytes+1)
	if _, err := DecodeVerifiedSpeechActSourceV2(basis, oversized, digest); err == nil ||
		!strings.Contains(err.Error(), "at most 262144 bytes") {
		t.Fatalf("over-limit canonical bytes error = %v", err)
	}
	boundaryJSON, _ := boundarySource.CanonicalJSON()
	boundaryDigest, _ := boundarySource.Digest()
	projection := speechActSourceV2Projection{}
	if err := json.Unmarshal(boundaryJSON, &projection); err != nil {
		t.Fatalf("decode boundary fixture: %v", err)
	}
	projection.AuditTraceRefs = append(projection.AuditTraceRefs, "audit:speech-act:overflow")
	overCollectionJSON, _ := json.Marshal(projection)
	if len(overCollectionJSON) > speechActSourceV2MaxCanonicalBytes {
		t.Fatal("collection-limit fixture accidentally exercises only the byte limit")
	}
	if _, err := DecodeVerifiedSpeechActSourceV2(
		basis,
		overCollectionJSON,
		boundaryDigest,
	); err == nil || !strings.Contains(err.Error(), "collections exceed reviewed decode limits") {
		t.Fatalf("over-limit decoded collection error = %v", err)
	}
}

type speechActSourceV2TestOption func(*speechActSourceV2TestOptions)

type speechActSourceV2TestOptions struct {
	description string
}

func withSpeechActSourceV2Description(value string) speechActSourceV2TestOption {
	return func(options *speechActSourceV2TestOptions) {
		options.description = value
	}
}

func testSpeechActSourceV2Anchors(
	t *testing.T,
	basis VerifiedSpeechActSource,
	workRaw string,
	carrierRaw string,
	options ...speechActSourceV2TestOption,
) SpeechActSourceV2Anchors {
	t.Helper()
	settings := speechActSourceV2TestOptions{
		description: basis.state.speechAct.state.reviewSubjectRef.String(),
	}
	for _, apply := range options {
		apply(&settings)
	}
	work := mustParse(t, NewWorkRef, workRaw)
	description := mustClaimDescriptionRef(t, settings.description)
	resource := mustParse(t, NewResourceLedgerRef, "resource-ledger:speech-act:test")
	acceptance := mustParse(t, NewAcceptancePostureRef, "acceptance:recognized:test")
	auditB := mustParse(t, NewAuditTraceRef, "audit:speech-act:b")
	auditA := mustParse(t, NewAuditTraceRef, "audit:speech-act:a")
	carrierRef := mustParse(t, NewCarrierRef, carrierRaw)
	carrierDigest := mustParse(t, NewDigest, testAuthorityDigest("a"))
	carrier, err := NewObservableCarrierBinding(carrierRef, carrierDigest)
	if err != nil {
		t.Fatalf("NewObservableCarrierBinding: %v", err)
	}
	anchors, err := NewSpeechActSourceV2AnchorsBuilder(work, description).
		WithResourceLedger(resource).
		WithAcceptancePosture(acceptance).
		WithAuditTrace(auditB).
		WithAuditTrace(auditA).
		WithDescriptionCarrier(carrier).
		Build()
	if err != nil {
		t.Fatalf("build SpeechAct source v2 anchors: %v", err)
	}
	return anchors
}

func mustClaimDescriptionRef(t *testing.T, raw string) DescriptionRef {
	t.Helper()
	ref, err := NewClaimIDDescriptionRef(raw)
	if err != nil {
		t.Fatalf("NewClaimIDDescriptionRef(%q): %v", raw, err)
	}
	return ref
}

func testSpeechActSourceWithCollapsedSpeechAndCaptureRefs(
	t *testing.T,
) VerifiedSpeechActSource {
	t.Helper()
	template := testVerifiedAuthorityAct(t).state.source.state.intent.state
	collapsedCapture := mustParse(t, NewCarrierRef, template.speechActRef.String())
	intent, err := NewPreparedSpeechActIntentBuilder(
		template.speechActRef,
		collapsedCapture,
	).
		ForProject(template.projectRoot).
		InSession(template.sessionRef).
		Reviewing(template.reviewSubjectRef, template.reviewSubjectDig).
		Institutes(template.institutedObject).
		UnderContextPolicy(template.contextPolicy).
		WithExecutionFrame(template.executionFrame).
		Build()
	if err != nil {
		t.Fatalf("build collapsed v1 intent: %v", err)
	}
	reviewText := "Observe one generic communicative Work occurrence."
	reviewDigest, err := SpeechActIntentReviewDigest(intent, reviewText)
	if err != nil {
		t.Fatalf("SpeechActIntentReviewDigest: %v", err)
	}
	prepared, err := PrepareManualSpeechAct(intent, reviewText)
	if err != nil {
		t.Fatalf("PrepareManualSpeechAct: %v", err)
	}
	now := canonicalAuthorityTime(time.Now())
	source, err := CaptureVerifiedSpeechActForTestFixture(
		t,
		prepared,
		now,
		now.Add(time.Millisecond),
		now.Add(2*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("CaptureVerifiedSpeechActForTestFixture(%s): %v", reviewDigest.String(), err)
	}
	return source
}

func testAuthorityDigest(hexDigit string) string {
	return "sha256:" + strings.Repeat(hexDigit, 64)
}

func addSpeechActSourceV2AuditFixtures(
	t *testing.T,
	builder SpeechActSourceV2AnchorsBuilder,
	count int,
	index int,
) SpeechActSourceV2AnchorsBuilder {
	t.Helper()
	if index >= count {
		return builder
	}
	ref := mustParse(
		t,
		NewAuditTraceRef,
		fmt.Sprintf("audit:speech-act:limit:%03d", index),
	)
	next := builder.WithAuditTrace(ref)
	return addSpeechActSourceV2AuditFixtures(t, next, count, index+1)
}

func addSpeechActSourceV2CarrierFixtures(
	t *testing.T,
	builder SpeechActSourceV2AnchorsBuilder,
	count int,
	index int,
) SpeechActSourceV2AnchorsBuilder {
	t.Helper()
	if index >= count {
		return builder
	}
	ref := mustParse(
		t,
		NewCarrierRef,
		fmt.Sprintf("carrier:speech-act:limit:%03d", index),
	)
	digestDigit := fmt.Sprintf("%x", index%16)
	digest := mustParse(t, NewDigest, testAuthorityDigest(digestDigit))
	binding, err := NewObservableCarrierBinding(ref, digest)
	if err != nil {
		t.Fatalf("NewObservableCarrierBinding: %v", err)
	}
	next := builder.WithDescriptionCarrier(binding)
	return addSpeechActSourceV2CarrierFixtures(t, next, count, index+1)
}

func assertSpeechActSourceV2Projection(
	t *testing.T,
	basis VerifiedSpeechActSource,
	projection speechActSourceV2Projection,
) {
	t.Helper()
	act := basis.state.speechAct.state
	if projection.Schema != speechActSourceV2Schema {
		t.Fatalf("schema = %q", projection.Schema)
	}
	if projection.SpeechActRef != act.ref.String() || projection.WorkRef == projection.SpeechActRef {
		t.Fatal("projection collapsed SpeechActRef and WorkRef")
	}
	if projection.Description.Ref != act.reviewSubjectRef.String() ||
		projection.Description.Digest != act.reviewSubjectDigest.String() {
		t.Fatal("projection does not bind the exact reviewed DescriptionRef")
	}
	if projection.Description.Ref == projection.TerminalCaptureRef ||
		projection.DescriptionCarriers[0].Ref == projection.TerminalCaptureRef {
		t.Fatal("projection collapsed description or content carrier with terminal capture")
	}
	if projection.ResourceLedgerRef == "" || projection.OutcomeRef != act.outcome.String() ||
		projection.AcceptancePostureRef == "" || len(projection.AuditTraceRefs) == 0 {
		t.Fatal("projection omitted reliance-bearing A.15.1 anchors")
	}
	if projection.AuditTraceRefs[0] != "audit:speech-act:a" {
		t.Fatal("audit-trace refs are not canonicalized")
	}
}

func assertSpeechActSourceV2Accessors(t *testing.T, source VerifiedSpeechActSourceV2) {
	t.Helper()
	speechAct, speechOK := source.SpeechActRef()
	work, workOK := source.WorkRef()
	description, descriptionOK := source.DescriptionRef()
	carriers, carriersOK := source.DescriptionCarriers()
	capture, captureOK := source.TerminalCaptureRef()
	resource, resourceOK := source.ResourceLedgerRef()
	outcome, outcomeOK := source.OutcomeRef()
	acceptance, acceptanceOK := source.AcceptancePostureRef()
	audit, auditOK := source.AuditTraceRefs()
	method, methodOK := source.MethodDescription()
	assignment, assignmentOK := source.PerformedByRoleAssignment()
	executedWithin, systemOK := source.ExecutedWithin()
	contextRef, contextOK := source.BoundedContext()
	window, windowOK := source.WorkWindow()
	parameters, parametersOK := source.ParameterBindings()
	inputs, inputsOK := source.InputRefs()
	outputs, outputsOK := source.OutputRefs()
	affected, affectedOK := source.AffectedRefs()
	statePlane, statePlaneOK := source.StatePlaneRef()
	delta, deltaOK := source.DeltaPredicateRef()
	policy, policyOK := source.ContextPolicy()
	allPresent := speechOK && workOK && descriptionOK && carriersOK && captureOK &&
		resourceOK && outcomeOK && acceptanceOK && auditOK && methodOK && assignmentOK &&
		systemOK && contextOK && windowOK && parametersOK && inputsOK && outputsOK &&
		affectedOK && statePlaneOK && deltaOK && policyOK
	if !allPresent {
		t.Fatal("v2 source omitted typed accessors")
	}
	if speechAct.String() == work.String() || speechAct.String() == description.String() ||
		work.String() == description.String() || carriers[0].Ref() == capture {
		t.Fatal("typed v2 accessors collapsed distinct source roles")
	}
	if resource.String() == "" || outcome.String() == "" || acceptance.String() == "" || len(audit) == 0 {
		t.Fatal("typed v2 accessors omitted reliance-bearing Work anchors")
	}
	methodRef, methodRefOK := method.MethodRef()
	assignmentRef, assignmentRefOK := assignment.Ref()
	holder, holderOK := assignment.HolderSystemRef()
	holderKind, holderKindOK := assignment.AdmittedHolderKind()
	role, roleOK := assignment.RoleRef()
	assignmentContext, assignmentContextOK := assignment.BoundedContext()
	assignmentWindow, assignmentWindowOK := assignment.AssignmentWindow()
	performerComplete := methodRefOK && assignmentRefOK && holderOK && holderKindOK &&
		roleOK && assignmentContextOK && assignmentWindowOK
	if !performerComplete || holderKind != admittedHolderSystemKindValue ||
		assignmentContext != contextRef || !assignmentWindow.Contains(window.From()) {
		t.Fatal("v2 source does not expose its complete RoleAssignment and Method anchors")
	}
	if methodRef.String() == "" || assignmentRef.String() == "" || holder.String() == "" ||
		role.String() == "" || executedWithin.String() == "" || len(parameters) == 0 ||
		len(inputs) == 0 || len(outputs) == 0 || len(affected) == 0 ||
		statePlane.String() == "" || delta.String() == "" || !policy.valid() {
		t.Fatal("v2 source omitted its inspectable A.2.9/A.15.1 source anchors")
	}
}
