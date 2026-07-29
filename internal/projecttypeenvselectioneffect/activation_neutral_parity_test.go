package projecttypeenvselectioneffect

import (
	"bytes"
	"testing"

	"github.com/m0n0x41d/haft/internal/projecttypeenvactivation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
)

func TestActivationCanonicalWireMatchesNeutralCore(t *testing.T) {
	fixture := newEffectFixture(t, 1)
	delta, err := projecttypeenvactivation.DecodeDelta(
		fixture.delta.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("neutral DecodeDelta(effect bytes): %v", err)
	}
	envelope, err := projecttypeenvactivation.DecodeAdmissionEnvelope(
		fixture.envelope.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("neutral DecodeAdmissionEnvelope(effect bytes): %v", err)
	}
	basis, err := projecttypeenvactivation.DecodeAdmissionBasis(
		fixture.basis.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("neutral DecodeAdmissionBasis(effect bytes): %v", err)
	}
	manifest, err := projecttypeenvactivation.DecodeMaterializationManifest(
		fixture.manifest.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("neutral DecodeMaterializationManifest(effect bytes): %v", err)
	}
	if err := projecttypeenvactivation.VerifyClosure(
		delta,
		envelope,
		basis,
		manifest,
	); err != nil {
		t.Fatalf("neutral VerifyClosure(effect carriers): %v", err)
	}
	checks := []struct {
		name          string
		effectBytes   []byte
		neutralBytes  []byte
		effectRef     string
		neutralRef    string
		effectDigest  string
		neutralDigest string
		goldenRef     string
	}{
		{
			name:          "delta",
			effectBytes:   fixture.delta.CanonicalBytes(),
			neutralBytes:  delta.CanonicalBytes(),
			effectRef:     fixture.delta.Ref().String(),
			neutralRef:    delta.Ref().String(),
			effectDigest:  fixture.delta.Digest().String(),
			neutralDigest: delta.Digest().String(),
			goldenRef: "project-typeenv-activation-delta:" +
				"sha256:5b945c2988434e94e2e86db7a45a260" +
				"2c1268ba1bba342a9a2f5f51f7a7c8674",
		},
		{
			name:          "envelope",
			effectBytes:   fixture.envelope.CanonicalBytes(),
			neutralBytes:  envelope.CanonicalBytes(),
			effectRef:     fixture.envelope.Ref().String(),
			neutralRef:    envelope.Ref().String(),
			effectDigest:  fixture.envelope.Digest().String(),
			neutralDigest: envelope.Digest().String(),
			goldenRef: "project-typeenv-activation-envelope:" +
				"sha256:50791e1dcb33fda945e58244dd63b228" +
				"d63d0ceae9d779053b0e6e026494253d",
		},
		{
			name:          "basis",
			effectBytes:   fixture.basis.CanonicalBytes(),
			neutralBytes:  basis.CanonicalBytes(),
			effectRef:     fixture.basis.Ref().String(),
			neutralRef:    basis.Ref().String(),
			effectDigest:  fixture.basis.Digest().String(),
			neutralDigest: basis.Digest().String(),
			goldenRef: "project-typeenv-activation-basis:" +
				"sha256:8b2b0c0c60282101f5d57a8b1fa8875" +
				"d37871f2b7369532ac5a9804d0b677926",
		},
		{
			name:          "manifest",
			effectBytes:   fixture.manifest.CanonicalBytes(),
			neutralBytes:  manifest.CanonicalBytes(),
			effectRef:     fixture.manifest.Ref().String(),
			neutralRef:    manifest.Ref().String(),
			effectDigest:  fixture.manifest.Digest().String(),
			neutralDigest: manifest.Digest().String(),
			goldenRef: "project-typeenv-activation-manifest:" +
				"sha256:55fcad6a7f244d207e07d959d7dc6b78" +
				"480ddcf56c04487de46df538a7ffdb0b",
		},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if !bytes.Equal(check.effectBytes, check.neutralBytes) {
				t.Fatalf("canonical bytes differ")
			}
			if check.effectRef != check.neutralRef {
				t.Fatalf(
					"ref = %q; neutral ref = %q",
					check.effectRef,
					check.neutralRef,
				)
			}
			if check.effectDigest != check.neutralDigest {
				t.Fatalf(
					"digest = %q; neutral digest = %q",
					check.effectDigest,
					check.neutralDigest,
				)
			}
			if check.effectRef != check.goldenRef {
				t.Fatalf(
					"ref = %q; golden ref = %q",
					check.effectRef,
					check.goldenRef,
				)
			}
		})
	}
}

func TestActivationLegacyV1DecodeIsReadOnly(t *testing.T) {
	fixture := newEffectFixture(t, 1)
	proof, err := projecttypeenvselection.ParseNoPriorHeadProofRef(
		"project-typeenv-no-prior-head-proof:" + mustTypedDigest(t, '7').String(),
	)
	if err != nil {
		t.Fatalf("ParseNoPriorHeadProofRef: %v", err)
	}
	deltaWriter := newCanonicalWriter(
		"haft.project-typeenv.activation-delta.v1",
	)
	deltaWriter.writeString(fixture.delta.TransactionRef().String())
	deltaWriter.writeString(fixture.delta.TransactionDigest().String())
	deltaWriter.writeString(fixture.delta.Project().String())
	deltaWriter.writeString(fixture.delta.Head().String())
	deltaWriter.writeString(fixture.delta.RequestRef().String())
	deltaWriter.writeString(fixture.delta.RequestDigest().String())
	deltaWriter.writeString(fixture.delta.ContentDigest().String())
	deltaWriter.writeString(fixture.delta.AuthorityUseRecordRef().String())
	deltaWriter.writeString(fixture.delta.WorkRef().String())
	deltaWriter.writeString(fixture.delta.CASWorkRecordRef().String())
	deltaWriter.writeString("genesis")
	deltaWriter.writeString(proof.String())
	encodeTarget(&deltaWriter, fixture.delta.Target())
	deltaWriter.writeUint64(fixture.delta.ExpectedGraphRevision().Value())
	deltaWriter.writeUint64(fixture.delta.CommittedGraphRevision().Value())
	deltaWriter.writeUint64(fixture.delta.SuccessorHeadRevision().Value())
	deltaWriter.writeString(ProjectTypeEnvActivationEventKind)
	deltaWriter.writeString(ProjectTypeEnvActivationAuthorityClass)
	legacyDeltaBytes := deltaWriter.bytes()
	legacyDelta, err := DecodeProjectTypeEnvActivationDelta(legacyDeltaBytes)
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvActivationDelta(v1): %v", err)
	}
	if !bytes.Equal(legacyDelta.CanonicalBytes(), legacyDeltaBytes) {
		t.Fatal("legacy delta canonical bytes changed")
	}
	if _, ok := legacyDelta.Predecessor().(projecttypeenvselection.GenesisStagePredecessor); !ok {
		t.Fatalf(
			"legacy public predecessor = %T; want tag-only Genesis",
			legacyDelta.Predecessor(),
		)
	}
	if _, err := SealProjectTypeEnvActivationAdmissionEnvelope(
		legacyDelta,
		fixture.dag,
	); err == nil {
		t.Fatal("legacy delta reissued a current envelope")
	}

	envelopeWriter := newCanonicalWriter(activationEnvelopeDomain)
	envelopeWriter.writeString(
		ProjectTypeEnvActivationAdmissionKindSnapshotOnly,
	)
	envelopeWriter.writeString(legacyDelta.Ref().String())
	envelopeWriter.writeString(legacyDelta.Digest().String())
	envelopeWriter.writeString(legacyDelta.RequestRef().String())
	envelopeWriter.writeString(legacyDelta.RequestDigest().String())
	envelopeWriter.writeString(legacyDelta.Target().Composite().String())
	envelopeWriter.writeString(legacyDelta.Target().Stage().String())
	envelopeWriter.writeString(fixture.dag.GraphIdempotencyKey().String())
	legacyEnvelope, err := DecodeProjectTypeEnvActivationAdmissionEnvelope(
		envelopeWriter.bytes(),
	)
	if err != nil {
		t.Fatalf(
			"DecodeProjectTypeEnvActivationAdmissionEnvelope(v1): %v",
			err,
		)
	}
	basisWriter := newCanonicalWriter(
		"haft.project-typeenv.activation-admission-basis.v1",
	)
	basisWriter.writeString(
		ProjectTypeEnvActivationAdmissionKindSnapshotOnly,
	)
	basisWriter.writeString(legacyEnvelope.Ref().String())
	basisWriter.writeString(legacyEnvelope.Digest().String())
	basisWriter.writeString(legacyDelta.Project().String())
	basisWriter.writeString("genesis")
	basisWriter.writeString(proof.String())
	basisWriter.writeString(legacyDelta.Target().Composite().String())
	basisWriter.writeString(legacyDelta.Target().Stage().String())
	basisWriter.writeUint64(legacyDelta.ExpectedGraphRevision().Value())
	legacyBasisBytes := basisWriter.bytes()
	legacyBasis, err := DecodeProjectTypeEnvActivationAdmissionBasis(
		legacyBasisBytes,
	)
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvActivationAdmissionBasis(v1): %v", err)
	}
	if !bytes.Equal(legacyBasis.CanonicalBytes(), legacyBasisBytes) {
		t.Fatal("legacy basis canonical bytes changed")
	}
	if _, err := SealProjectTypeEnvActivationAdmissionBasis(
		legacyDelta,
		legacyEnvelope,
	); err == nil {
		t.Fatal("legacy admission reissued a current basis")
	}
}
