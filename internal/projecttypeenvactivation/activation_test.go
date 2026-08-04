package projecttypeenvactivation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type activationCarrierFixture struct {
	delta    Delta
	envelope AdmissionEnvelope
	basis    AdmissionBasis
	manifest MaterializationManifest
}

func TestActivationCarrierClosureStrictRoundTrip(t *testing.T) {
	fixture := newActivationCarrierFixture(t, '1')
	if err := VerifyClosure(
		fixture.delta,
		fixture.envelope,
		fixture.basis,
		fixture.manifest,
	); err != nil {
		t.Fatalf("VerifyClosure: %v", err)
	}
	roundTrips := []struct {
		name      string
		canonical []byte
		decode    func([]byte) ([]byte, error)
	}{
		{
			name:      "delta",
			canonical: fixture.delta.CanonicalBytes(),
			decode: func(value []byte) ([]byte, error) {
				decoded, err := DecodeDelta(value)
				return decoded.CanonicalBytes(), err
			},
		},
		{
			name:      "envelope",
			canonical: fixture.envelope.CanonicalBytes(),
			decode: func(value []byte) ([]byte, error) {
				decoded, err := DecodeAdmissionEnvelope(value)
				return decoded.CanonicalBytes(), err
			},
		},
		{
			name:      "basis",
			canonical: fixture.basis.CanonicalBytes(),
			decode: func(value []byte) ([]byte, error) {
				decoded, err := DecodeAdmissionBasis(value)
				return decoded.CanonicalBytes(), err
			},
		},
		{
			name:      "manifest",
			canonical: fixture.manifest.CanonicalBytes(),
			decode: func(value []byte) ([]byte, error) {
				decoded, err := DecodeMaterializationManifest(value)
				return decoded.CanonicalBytes(), err
			},
		},
	}
	for _, roundTrip := range roundTrips {
		t.Run(roundTrip.name, func(t *testing.T) {
			decoded, err := roundTrip.decode(roundTrip.canonical)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !bytes.Equal(decoded, roundTrip.canonical) {
				t.Fatalf("round-trip bytes differ")
			}
			trailing := append(append([]byte(nil), roundTrip.canonical...), 0)
			if _, err := roundTrip.decode(trailing); err == nil {
				t.Fatalf("trailing byte was accepted")
			}
		})
	}
}

func TestActivationCarrierClosureRejectsCrossMemberSubstitution(t *testing.T) {
	left := newActivationCarrierFixture(t, '2')
	right := newActivationCarrierFixture(t, '3')
	if err := VerifyEnvelopeForDelta(left.delta, right.envelope); err == nil {
		t.Fatalf("foreign envelope was accepted")
	}
	if err := VerifyAdmission(left.delta, left.envelope, right.basis); err == nil {
		t.Fatalf("foreign basis was accepted")
	}
	if err := VerifyClosure(
		left.delta,
		left.envelope,
		left.basis,
		right.manifest,
	); err == nil {
		t.Fatalf("foreign manifest was accepted")
	}
}

func TestActivationCarrierDigestIsDomainSeparatedFromRawBytes(t *testing.T) {
	fixture := newActivationCarrierFixture(t, '4')
	raw, err := rawActivationTestDigest(fixture.delta.CanonicalBytes())
	if err != nil {
		t.Fatalf("raw digest: %v", err)
	}
	if raw == fixture.delta.Digest() {
		t.Fatalf("activation digest collapsed to a raw canonical-byte digest")
	}
}

func TestTransitionDeltaRejectsNoOpAndNonContiguousHeadRevision(t *testing.T) {
	fixture := newActivationCarrierFixture(t, '5')
	priorRevision, err := projecttypeenvselection.NewHeadRevision(7)
	if err != nil {
		t.Fatalf("NewHeadRevision(prior): %v", err)
	}
	predecessor, err := projecttypeenvselection.NewTransitionStagePredecessor(
		projecttypeenvselection.TransitionStagePredecessorInput{
			Project:           fixture.delta.Project(),
			Head:              fixture.delta.Head(),
			HeadRevision:      priorRevision,
			SelectedComposite: fixture.delta.Target().Composite(),
		},
	)
	if err != nil {
		t.Fatalf("NewTransitionStagePredecessor: %v", err)
	}
	successor, err := projecttypeenvselection.NewHeadRevision(8)
	if err != nil {
		t.Fatalf("NewHeadRevision(successor): %v", err)
	}
	input := deltaInputFromFixture(fixture.delta)
	input.Predecessor = predecessor
	input.SuccessorHeadRevision = successor
	if _, err := NewDelta(input); err == nil {
		t.Fatalf("Transition no-op activation was accepted")
	}

	priorComposite := activationTestTypeEnvRef(t, 'e')
	predecessor, err = projecttypeenvselection.NewTransitionStagePredecessor(
		projecttypeenvselection.TransitionStagePredecessorInput{
			Project:           fixture.delta.Project(),
			Head:              fixture.delta.Head(),
			HeadRevision:      priorRevision,
			SelectedComposite: priorComposite,
		},
	)
	if err != nil {
		t.Fatalf("NewTransitionStagePredecessor(prior C): %v", err)
	}
	nonContiguous, err := projecttypeenvselection.NewHeadRevision(9)
	if err != nil {
		t.Fatalf("NewHeadRevision(non-contiguous): %v", err)
	}
	input.Predecessor = predecessor
	input.SuccessorHeadRevision = nonContiguous
	if _, err := NewDelta(input); err == nil {
		t.Fatalf("non-contiguous Transition HeadRevision was accepted")
	}
	input.SuccessorHeadRevision = successor
	if _, err := NewDelta(input); err != nil {
		t.Fatalf("contiguous Transition activation rejected: %v", err)
	}
}

func TestGenesisDeltaRequiresFirstHeadRevision(t *testing.T) {
	fixture := newActivationCarrierFixture(t, '6')
	successor, err := projecttypeenvselection.NewHeadRevision(2)
	if err != nil {
		t.Fatalf("NewHeadRevision: %v", err)
	}
	input := deltaInputFromFixture(fixture.delta)
	input.SuccessorHeadRevision = successor
	if _, err := NewDelta(input); err == nil {
		t.Fatalf("Genesis successor HeadRevision greater than one was accepted")
	}
}

func TestCurrentGenesisCarriersUseTagOnlyVersionMap(t *testing.T) {
	fixture := newActivationCarrierFixture(t, '7')
	if fixture.delta.edition != currentCarrierV3 ||
		fixture.basis.edition != currentCarrierV3 {
		t.Fatal("current constructors did not issue v3 delta and basis")
	}
	if _, err := newCanonicalReader(
		fixture.delta.CanonicalBytes(),
		deltaDomainV3,
	); err != nil {
		t.Fatalf("current delta domain: %v", err)
	}
	if _, err := newCanonicalReader(
		fixture.basis.CanonicalBytes(),
		basisDomainV3,
	); err != nil {
		t.Fatalf("current basis domain: %v", err)
	}
	if _, err := newCanonicalReader(
		fixture.envelope.CanonicalBytes(),
		envelopeDomain,
	); err != nil {
		t.Fatalf("current envelope domain: %v", err)
	}
	if _, err := newCanonicalReader(
		fixture.manifest.CanonicalBytes(),
		manifestDomain,
	); err != nil {
		t.Fatalf("current manifest domain: %v", err)
	}
	proofPrefix := []byte("project-typeenv-no-prior-head-proof:")
	if bytes.Contains(fixture.delta.CanonicalBytes(), proofPrefix) ||
		bytes.Contains(fixture.basis.CanonicalBytes(), proofPrefix) {
		t.Fatal("current Genesis carrier retained a no-prior-head proof")
	}
}

func TestLegacyV2GenesisCarriersRemainExactReadOnly(t *testing.T) {
	fixture := newActivationCarrierFixture(t, '6')
	deltaWriter := newCanonicalWriter(deltaDomainV2)
	deltaWriter.writeString(fixture.delta.TransactionRef())
	deltaWriter.writeString(fixture.delta.TransactionDigest().String())
	deltaWriter.writeString(fixture.delta.Project().String())
	deltaWriter.writeString(fixture.delta.Head().String())
	deltaWriter.writeString(fixture.delta.RequestRef().String())
	deltaWriter.writeString(fixture.delta.RequestDigest().String())
	deltaWriter.writeString(fixture.delta.ContentDigest().String())
	deltaWriter.writeString(fixture.delta.AuthorityUseRef())
	deltaWriter.writeString(fixture.delta.WorkRef().String())
	deltaWriter.writeString(fixture.delta.WorkRecordRef())
	deltaWriter.writeString("genesis")
	if err := encodeTarget(&deltaWriter, fixture.delta.Target()); err != nil {
		t.Fatalf("encodeTarget(legacy v2): %v", err)
	}
	deltaWriter.writeUint64(fixture.delta.ExpectedGraphRevision().Value())
	deltaWriter.writeUint64(fixture.delta.CommittedGraphRevision().Value())
	deltaWriter.writeUint64(fixture.delta.SuccessorHeadRevision().Value())
	deltaWriter.writeString(EventKind)
	deltaWriter.writeString(LegacyManualAuthorityClass)
	legacyDelta, err := DecodeDelta(deltaWriter.bytes())
	if err != nil {
		t.Fatalf("DecodeDelta(legacy v2): %v", err)
	}
	if legacyDelta.edition != legacyCarrierV2 ||
		legacyDelta.AuthorityClass() != LegacyManualAuthorityClass {
		t.Fatalf(
			"legacy v2 delta edition=%d authority=%q",
			legacyDelta.edition,
			legacyDelta.AuthorityClass(),
		)
	}
	if _, err := NewAdmissionEnvelope(
		legacyDelta,
		graphKeyPrefix+strings.Repeat("c", 64),
	); err == nil {
		t.Fatal("legacy v2 delta reissued a current envelope")
	}

	envelope := decodeLegacyEnvelopeFixture(
		t,
		legacyDelta,
		graphKeyPrefix+strings.Repeat("c", 64),
	)
	basisWriter := newCanonicalWriter(basisDomainV2)
	basisWriter.writeString(AdmissionKindSnapshotOnly)
	basisWriter.writeString(envelope.Ref().String())
	basisWriter.writeString(envelope.Digest().String())
	basisWriter.writeString(legacyDelta.Project().String())
	basisWriter.writeString("genesis")
	basisWriter.writeString(legacyDelta.Target().Composite().String())
	basisWriter.writeString(legacyDelta.Target().Stage().String())
	basisWriter.writeUint64(legacyDelta.ExpectedGraphRevision().Value())
	legacyBasis, err := DecodeAdmissionBasis(basisWriter.bytes())
	if err != nil {
		t.Fatalf("DecodeAdmissionBasis(legacy v2): %v", err)
	}
	if legacyBasis.edition != legacyCarrierV2 {
		t.Fatalf("legacy v2 basis edition = %d", legacyBasis.edition)
	}
}

func TestCurrentActivationRequiresExactCurrentAuthorityClass(t *testing.T) {
	fixture := newActivationCarrierFixture(t, '5')
	input := deltaInputFromFixture(fixture.delta)
	input.AuthorityClass = LegacyManualAuthorityClass
	if _, err := NewDelta(input); err == nil {
		t.Fatal("current activation accepted the legacy manual authority class")
	}
	input.AuthorityClass = CompatibleSuccessorPolicyAuthorityClass
	delta, err := NewDelta(input)
	if err != nil {
		t.Fatalf("NewDelta(compatible successor policy): %v", err)
	}
	if delta.AuthorityClass() != CompatibleSuccessorPolicyAuthorityClass {
		t.Fatalf("current activation authority class = %q", delta.AuthorityClass())
	}
}

func TestLegacyGenesisCarrierClosureRemainsExactReadOnly(t *testing.T) {
	fixture := newLegacyActivationCarrierFixture(t, '8')
	goldenDigests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "delta-v1",
			got:  fixture.delta.Digest().String(),
			want: "sha256:29a6f965d3aba4c3b1048fa64c250de35c168af95c8b3f3ece0a42c04092cc26",
		},
		{
			name: "envelope-v1",
			got:  fixture.envelope.Digest().String(),
			want: "sha256:524ba6fbfe4d5d1e934d3a5e7464fb5d5d4b3bd08247b011c1ee65d5d569e82b",
		},
		{
			name: "basis-v1",
			got:  fixture.basis.Digest().String(),
			want: "sha256:cc69917e10062608a68dd4e6e6446cf4c42bff04db4ad2dfc5b8233b2fe59c95",
		},
		{
			name: "manifest-v1",
			got:  fixture.manifest.Digest().String(),
			want: "sha256:0e02f965b2d7e35c3039aeef122d03cbe5de6e021cbbed7e69b6ff3924053852",
		},
	}
	for _, golden := range goldenDigests {
		if golden.got != golden.want {
			t.Fatalf(
				"%s digest = %s; want historical %s",
				golden.name,
				golden.got,
				golden.want,
			)
		}
	}
	if fixture.delta.edition != legacyCarrierV1 ||
		fixture.basis.edition != legacyCarrierV1 {
		t.Fatal("legacy decoders lost the v1 carrier edition")
	}
	if err := VerifyClosure(
		fixture.delta,
		fixture.envelope,
		fixture.basis,
		fixture.manifest,
	); err != nil {
		t.Fatalf("VerifyClosure(legacy): %v", err)
	}
	if _, ok := fixture.delta.Predecessor().(projecttypeenvselection.GenesisStagePredecessor); !ok {
		t.Fatalf(
			"legacy public predecessor = %T; want tag-only Genesis",
			fixture.delta.Predecessor(),
		)
	}
	foreignProof, err := projecttypeenvselection.ParseNoPriorHeadProofRef(
		"project-typeenv-no-prior-head-proof:" +
			activationTestDigest('f').String(),
	)
	if err != nil {
		t.Fatalf("ParseNoPriorHeadProofRef(foreign): %v", err)
	}
	foreignBasis := decodeLegacyBasisFixture(
		t,
		fixture.delta,
		fixture.envelope,
		foreignProof,
	)
	if err := VerifyAdmission(
		fixture.delta,
		fixture.envelope,
		foreignBasis,
	); err == nil {
		t.Fatal("legacy basis with a substituted proof matched the delta")
	}
	roundTrips := []struct {
		name      string
		canonical []byte
		decode    func([]byte) ([]byte, error)
	}{
		{
			name:      "delta-v1",
			canonical: fixture.delta.CanonicalBytes(),
			decode: func(value []byte) ([]byte, error) {
				decoded, err := DecodeDelta(value)
				return decoded.CanonicalBytes(), err
			},
		},
		{
			name:      "basis-v1",
			canonical: fixture.basis.CanonicalBytes(),
			decode: func(value []byte) ([]byte, error) {
				decoded, err := DecodeAdmissionBasis(value)
				return decoded.CanonicalBytes(), err
			},
		},
	}
	for _, roundTrip := range roundTrips {
		t.Run(roundTrip.name, func(t *testing.T) {
			decoded, err := roundTrip.decode(roundTrip.canonical)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !bytes.Equal(decoded, roundTrip.canonical) {
				t.Fatal("legacy canonical bytes changed")
			}
		})
	}
	graphKey := fixture.envelope.GraphIdempotencyKey()
	if _, err := NewAdmissionEnvelope(fixture.delta, graphKey); err == nil {
		t.Fatal("legacy delta reissued a current envelope")
	}
	if _, err := NewAdmissionBasis(
		fixture.delta,
		fixture.envelope,
	); err == nil {
		t.Fatal("legacy admission reissued a current basis")
	}
	if _, err := NewMaterializationManifest(
		fixture.delta,
		fixture.envelope,
		fixture.basis,
		fixture.manifest.EventRef(),
		fixture.manifest.CommitRef(),
	); err == nil {
		t.Fatal("legacy closure reissued a current manifest")
	}
}

func newActivationCarrierFixture(t *testing.T, fill byte) activationCarrierFixture {
	t.Helper()
	project, err := projectidentity.ParseProjectID("qnt_1234abcd")
	if err != nil {
		t.Fatalf("ParseProjectID: %v", err)
	}
	head, err := projecttypeenvselection.ProjectTypeEnvHeadRefForProject(project)
	if err != nil {
		t.Fatalf("ProjectTypeEnvHeadRefForProject: %v", err)
	}
	predecessor := projecttypeenvselection.NewGenesisStagePredecessor()
	base := activationTestTypeEnvRef(t, fill)
	composite := activationTestTypeEnvRef(t, fill+1)
	runtimeBasis, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(
		"runtime-evaluation-basis:" + activationTestDigest(fill+2).String(),
	)
	if err != nil {
		t.Fatalf("ParseRuntimeEvaluationBasisRef: %v", err)
	}
	stage, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(
		"project-typeenv-stage:" + activationTestDigest(fill+3).String(),
	)
	if err != nil {
		t.Fatalf("ParseProjectTypeEnvStageRef: %v", err)
	}
	target, err := NewTarget(TargetInput{
		Base:         base,
		RuntimeBasis: runtimeBasis,
		Composite:    composite,
		Stage:        stage,
	})
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		"project-typeenv-head-selection-request:" + activationTestDigest(fill+4).String(),
	)
	if err != nil {
		t.Fatalf("ParseProjectTypeEnvHeadSelectionRequestRef: %v", err)
	}
	work, err := authority.NewWorkRef("work:activation-test-" + string(fill))
	if err != nil {
		t.Fatalf("NewWorkRef: %v", err)
	}
	contentDigest, err := authority.NewDigest(activationTestDigest(fill + 5).String())
	if err != nil {
		t.Fatalf("NewDigest: %v", err)
	}
	transactionDigest := activationTestDigest(fill + 6)
	successorHead, err := projecttypeenvselection.NewHeadRevision(1)
	if err != nil {
		t.Fatalf("NewHeadRevision: %v", err)
	}
	delta, err := NewDelta(DeltaInput{
		TransactionRef:         transactionRefPrefix + transactionDigest.String(),
		TransactionDigest:      transactionDigest,
		Project:                project,
		Head:                   head,
		RequestRef:             requestRef,
		RequestDigest:          requestRef.Digest(),
		ContentDigest:          contentDigest,
		AuthorityUseRef:        authorityUseRefPrefix + activationTestDigest(fill+7).String(),
		WorkRef:                work,
		WorkRecordRef:          workRecordRefPrefix + activationTestDigest(fill+8).String(),
		Predecessor:            predecessor,
		Target:                 target,
		ExpectedGraphRevision:  typedmemory.NewGraphRevision(0),
		CommittedGraphRevision: typedmemory.NewGraphRevision(1),
		SuccessorHeadRevision:  successorHead,
		AuthorityClass:         HostRoutedOperatorRequestAuthorityClass,
	})
	if err != nil {
		t.Fatalf("NewDelta: %v", err)
	}
	graphKey := graphKeyPrefix + strings.Repeat(string(fill), 64)
	envelope, err := NewAdmissionEnvelope(delta, graphKey)
	if err != nil {
		t.Fatalf("NewAdmissionEnvelope: %v", err)
	}
	basis, err := NewAdmissionBasis(delta, envelope)
	if err != nil {
		t.Fatalf("NewAdmissionBasis: %v", err)
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(
		"typed-memory-event:" + strings.Repeat(string(fill), 64),
	)
	if err != nil {
		t.Fatalf("ParseGraphEventRef: %v", err)
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(
		"typed-memory-commit:" + strings.Repeat(string(fill+1), 64),
	)
	if err != nil {
		t.Fatalf("ParseGraphCommitRef: %v", err)
	}
	manifest, err := NewMaterializationManifest(delta, envelope, basis, event, commit)
	if err != nil {
		t.Fatalf("NewMaterializationManifest: %v", err)
	}
	return activationCarrierFixture{
		delta:    delta,
		envelope: envelope,
		basis:    basis,
		manifest: manifest,
	}
}

func newLegacyActivationCarrierFixture(
	t *testing.T,
	fill byte,
) activationCarrierFixture {
	t.Helper()
	current := newActivationCarrierFixture(t, fill)
	proof, err := projecttypeenvselection.ParseNoPriorHeadProofRef(
		"project-typeenv-no-prior-head-proof:" +
			activationTestDigest(fill+9).String(),
	)
	if err != nil {
		t.Fatalf("ParseNoPriorHeadProofRef: %v", err)
	}
	delta := decodeLegacyDeltaFixture(t, current.delta, proof)
	envelope := decodeLegacyEnvelopeFixture(
		t,
		delta,
		current.envelope.GraphIdempotencyKey(),
	)
	basis := decodeLegacyBasisFixture(t, delta, envelope, proof)
	manifest := decodeLegacyManifestFixture(
		t,
		delta,
		envelope,
		basis,
		current.manifest.EventRef(),
		current.manifest.CommitRef(),
	)
	return activationCarrierFixture{
		delta:    delta,
		envelope: envelope,
		basis:    basis,
		manifest: manifest,
	}
}

func decodeLegacyDeltaFixture(
	t *testing.T,
	source Delta,
	proof projecttypeenvselection.NoPriorHeadProofRef,
) Delta {
	t.Helper()
	writer := newCanonicalWriter(deltaDomainV1)
	writer.writeString(source.TransactionRef())
	writer.writeString(source.TransactionDigest().String())
	writer.writeString(source.Project().String())
	writer.writeString(source.Head().String())
	writer.writeString(source.RequestRef().String())
	writer.writeString(source.RequestDigest().String())
	writer.writeString(source.ContentDigest().String())
	writer.writeString(source.AuthorityUseRef())
	writer.writeString(source.WorkRef().String())
	writer.writeString(source.WorkRecordRef())
	writer.writeString("genesis")
	writer.writeString(proof.String())
	if err := encodeTarget(&writer, source.Target()); err != nil {
		t.Fatalf("encodeTarget(legacy v1): %v", err)
	}
	writer.writeUint64(source.ExpectedGraphRevision().Value())
	writer.writeUint64(source.CommittedGraphRevision().Value())
	writer.writeUint64(source.SuccessorHeadRevision().Value())
	writer.writeString(EventKind)
	writer.writeString(AuthorityClass)
	delta, err := DecodeDelta(writer.bytes())
	if err != nil {
		t.Fatalf("DecodeDelta(legacy v1): %v", err)
	}
	return delta
}

func decodeLegacyEnvelopeFixture(
	t *testing.T,
	delta Delta,
	graphKey string,
) AdmissionEnvelope {
	t.Helper()
	writer := newCanonicalWriter(envelopeDomain)
	writer.writeString(AdmissionKindSnapshotOnly)
	writer.writeString(delta.Ref().String())
	writer.writeString(delta.Digest().String())
	writer.writeString(delta.RequestRef().String())
	writer.writeString(delta.RequestDigest().String())
	writer.writeString(delta.Target().Composite().String())
	writer.writeString(delta.Target().Stage().String())
	writer.writeString(graphKey)
	envelope, err := DecodeAdmissionEnvelope(writer.bytes())
	if err != nil {
		t.Fatalf("DecodeAdmissionEnvelope(legacy): %v", err)
	}
	return envelope
}

func decodeLegacyBasisFixture(
	t *testing.T,
	delta Delta,
	envelope AdmissionEnvelope,
	proof projecttypeenvselection.NoPriorHeadProofRef,
) AdmissionBasis {
	t.Helper()
	writer := newCanonicalWriter(basisDomainV1)
	writer.writeString(AdmissionKindSnapshotOnly)
	writer.writeString(envelope.Ref().String())
	writer.writeString(envelope.Digest().String())
	writer.writeString(delta.Project().String())
	writer.writeString("genesis")
	writer.writeString(proof.String())
	writer.writeString(delta.Target().Composite().String())
	writer.writeString(delta.Target().Stage().String())
	writer.writeUint64(delta.ExpectedGraphRevision().Value())
	basis, err := DecodeAdmissionBasis(writer.bytes())
	if err != nil {
		t.Fatalf("DecodeAdmissionBasis(legacy v1): %v", err)
	}
	return basis
}

func decodeLegacyManifestFixture(
	t *testing.T,
	delta Delta,
	envelope AdmissionEnvelope,
	basis AdmissionBasis,
	event projecttypeenvselection.GraphEventRef,
	commit projecttypeenvselection.GraphCommitRef,
) MaterializationManifest {
	t.Helper()
	writer := newCanonicalWriter(manifestDomain)
	writer.writeString(delta.Ref().String())
	writer.writeString(delta.Digest().String())
	writer.writeString(envelope.Ref().String())
	writer.writeString(envelope.Digest().String())
	writer.writeString(basis.Ref().String())
	writer.writeString(basis.Digest().String())
	writer.writeString(event.String())
	writer.writeString(commit.String())
	writer.writeUint32(MaterializationOrdinal)
	writer.writeUint32(1)
	writer.writeUint32(1)
	writer.writeString(delta.Digest().String())
	manifest, err := DecodeMaterializationManifest(writer.bytes())
	if err != nil {
		t.Fatalf("DecodeMaterializationManifest(legacy): %v", err)
	}
	return manifest
}

func activationTestDigest(fill byte) typedmemory.SHA256Digest {
	digit := "0123456789abcdef"[int(fill)%16]
	value, _ := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat(string(digit), 64),
	)
	return value
}

func activationTestTypeEnvRef(t *testing.T, fill byte) typedmemory.TypeEnvRef {
	t.Helper()
	value, err := typedmemory.NewTypeEnvRef(activationTestDigest(fill))
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	return value
}

func deltaInputFromFixture(delta Delta) DeltaInput {
	return DeltaInput{
		TransactionRef:         delta.TransactionRef(),
		TransactionDigest:      delta.TransactionDigest(),
		Project:                delta.Project(),
		Head:                   delta.Head(),
		RequestRef:             delta.RequestRef(),
		RequestDigest:          delta.RequestDigest(),
		ContentDigest:          delta.ContentDigest(),
		AuthorityUseRef:        delta.AuthorityUseRef(),
		WorkRef:                delta.WorkRef(),
		WorkRecordRef:          delta.WorkRecordRef(),
		Predecessor:            delta.Predecessor(),
		Target:                 delta.Target(),
		ExpectedGraphRevision:  delta.ExpectedGraphRevision(),
		CommittedGraphRevision: delta.CommittedGraphRevision(),
		SuccessorHeadRevision:  delta.SuccessorHeadRevision(),
		AuthorityClass:         delta.AuthorityClass(),
	}
}

func rawActivationTestDigest(value []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(value)
	return typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
}
