package projecttypeenvassertionrevalidation

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type passthroughCodec struct{}

func (passthroughCodec) Canonicalize(
	_ typedmemory.ValueShapeRef,
	input []byte,
) typedmemory.CodecCanonicalization {
	return mustTestValue(typedmemory.NewCanonicalizedCodecValue(
		typedmemory.NewBytesValue(input),
		input,
	))
}

type changingCodec struct{}

func (changingCodec) Canonicalize(
	_ typedmemory.ValueShapeRef,
	input []byte,
) typedmemory.CodecCanonicalization {
	canonical := append([]byte(nil), input...)
	if len(canonical) > 0 && canonical[len(canonical)-1] != '!' {
		canonical = append(canonical, '!')
	}
	return mustTestValue(typedmemory.NewCanonicalizedCodecValue(
		typedmemory.NewBytesValue(canonical),
		canonical,
	))
}

type revalidationFixture struct {
	target    typedmemory.TypeEnv
	runtime   projecttypeenvruntime.ExactTargetRuntimeRegistry
	codec     typedmemory.CodecRef
	context   typedmemory.BoundedContextRef
	signature typedmemory.RelationSignatureRef
	slot      typedmemory.SlotKindID
	valueKind typedmemory.ValueKindRef
	shape     typedmemory.ValueShapeRef
}

func TestRevalidateEmptyGraphIsCleanRoundTripsAndIsImmutable(t *testing.T) {
	t.Parallel()
	fixture := newRevalidationFixture(t, 0x11, passthroughCodec{})
	observation := emptyObservation(t, fixture.target.Ref(), "qnt_1234abcd")

	report, err := Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: fixture.target,
		TargetRuntime: fixture.runtime,
	})
	if err != nil {
		t.Fatalf("Revalidate() error = %v", err)
	}
	if report.Posture() != typedmemory.RevalidationClean {
		t.Fatalf("Posture() = %s, want clean", report.Posture().String())
	}
	if len(report.Outcomes()) != 0 ||
		len(report.AffectedAssertions()) != 0 {
		t.Fatal("empty graph produced assertion outcomes")
	}
	graphSnapshot := report.GraphSnapshot()
	if graphSnapshot.Ref().String() !=
		observation.GraphSnapshotBasis().Ref().String() ||
		graphSnapshot.Revision() !=
			observation.GraphSnapshotBasis().GraphRevision() ||
		graphSnapshot.BasisDigest() !=
			observation.GraphSnapshotBasis().Ref().Digest() {
		t.Fatal("complete report lost the exact graph snapshot coordinate")
	}
	if err := report.Verify(); err != nil {
		t.Fatalf("Report.Verify() error = %v", err)
	}
	decoded, err := DecodeCanonicalReport(report.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeCanonicalReport() error = %v", err)
	}
	if decoded.Digest() != report.Digest() {
		t.Fatal("canonical round-trip changed report digest")
	}

	bytesView := report.CanonicalBytes()
	bytesView[0] ^= 0xff
	outcomesView := report.Outcomes()
	outcomesView = append(outcomesView, nil)
	affectedView := report.AffectedAssertions()
	affectedView = append(
		affectedView,
		mustTestValue(typedmemory.NewAssertionID("assertion:mutated")),
	)
	if bytes.Equal(bytesView, report.CanonicalBytes()) ||
		len(report.Outcomes()) != 0 ||
		len(report.AffectedAssertions()) != 0 {
		t.Fatal("report accessors exposed mutable state")
	}
}

func TestRevalidateUsesTargetSignatureAndFailsClosedWhenItIsAbsent(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRevalidationFixture(t, 0x21, passthroughCodec{})
	relation := valueRelation(
		t,
		fixture,
		"assertion:absent-signature",
		fixture.signature,
		1,
	)
	targetWithoutSignature := typeEnvWithContextOnly(
		t,
		fixture.target.Ref(),
		fixture.context,
		0x22,
	)
	observation := committedObservation(
		t,
		targetWithoutSignature.Ref(),
		"qnt_1234abcd",
		[]typedmemory.RelationInstance{relation},
	)
	report, err := Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: targetWithoutSignature,
		TargetRuntime: fixture.runtime,
	})
	if err != nil {
		t.Fatalf("Revalidate() error = %v", err)
	}
	if report.Posture() != typedmemory.RevalidationUnderdetermined {
		t.Fatalf(
			"Posture() = %s, want underdetermined",
			report.Posture().String(),
		)
	}
	outcome := report.Outcomes()[0]
	if outcome.Kind() != AssertionUnderdetermined ||
		!hasGroundCode(outcome, CodeTargetRelationFragmentUnavailable) {
		t.Fatalf(
			"outcome = %s, grounds = %#v",
			outcome.Kind().String(),
			outcome.Grounds(),
		)
	}
}

func TestRevalidateValueRelationCleanAndCodecReinterpretationConflicts(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRevalidationFixture(t, 0x31, passthroughCodec{})
	relation := valueRelation(
		t,
		fixture,
		"assertion:value",
		fixture.signature,
		1,
	)
	observation := committedObservation(
		t,
		fixture.target.Ref(),
		"qnt_1234abcd",
		[]typedmemory.RelationInstance{relation},
	)
	clean, err := Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: fixture.target,
		TargetRuntime: fixture.runtime,
	})
	if err != nil {
		t.Fatalf("clean Revalidate() error = %v", err)
	}
	if clean.Posture() != typedmemory.RevalidationClean ||
		clean.Outcomes()[0].Kind() != AssertionValid {
		t.Fatalf(
			"clean report = %s/%s",
			clean.Posture().String(),
			clean.Outcomes()[0].Kind().String(),
		)
	}

	changingRuntime := exactRuntime(
		t,
		fixture.codec,
		changingCodec{},
		"artifact:runtime-changing",
	)
	conflict, err := Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: fixture.target,
		TargetRuntime: changingRuntime,
	})
	if err != nil {
		t.Fatalf("conflict Revalidate() error = %v", err)
	}
	if conflict.Posture() != typedmemory.RevalidationConflict ||
		conflict.Outcomes()[0].Kind() != AssertionInvalid ||
		!hasGroundCode(
			conflict.Outcomes()[0],
			CodeValueCanonicalBytesChanged,
		) {
		t.Fatalf(
			"conflict report = %s/%s grounds=%#v",
			conflict.Posture().String(),
			conflict.Outcomes()[0].Kind().String(),
			conflict.Outcomes()[0].Grounds(),
		)
	}
	if clean.Digest() == conflict.Digest() {
		t.Fatal("changed runtime outcome did not change report digest")
	}
}

func TestRevalidateV3AssertionUsesExactCarrierWithoutOccurrenceInference(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRevalidationFixture(t, 0x35, passthroughCodec{})
	legacyShape := valueRelation(
		t,
		fixture,
		"assertion:v3-value",
		fixture.signature,
		1,
	)
	assertion := relationalAssertionFromRelation(
		t,
		legacyShape,
		typedmemory.AssertionModalityDeniesObtaining,
	)
	observation := committedRelationalAssertionObservation(
		t,
		fixture.target.Ref(),
		"qnt_1234abcd",
		assertion,
	)
	active := observation.ActiveAssertions().Relations()[0]
	if _, legacy := active.LegacyRelation(); legacy {
		t.Fatal("v3 revalidation fixture inferred a legacy relation occurrence")
	}
	modality, explicit := active.Posture().ExplicitModality()
	if !explicit || modality != typedmemory.AssertionModalityDeniesObtaining {
		t.Fatal("v3 revalidation fixture lost the explicit denial posture")
	}

	report, err := Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: fixture.target,
		TargetRuntime: fixture.runtime,
	})
	if err != nil {
		t.Fatalf("Revalidate(v3) error = %v", err)
	}
	if report.Posture() != typedmemory.RevalidationClean ||
		len(report.Outcomes()) != 1 {
		t.Fatalf(
			"v3 report = (%s, %d outcomes); want one clean structural outcome",
			report.Posture().String(),
			len(report.Outcomes()),
		)
	}
	digest := mustTestValue(assertion.Digest())
	if report.Outcomes()[0].RelationDigest() != digest {
		t.Fatal("v3 revalidation report lost the exact assertion digest")
	}
}

func TestRevalidateRequiresAppendOnlyMigrationWhenCodecCoordinateChanges(
	t *testing.T,
) {
	t.Parallel()
	storedFixture := newRevalidationFixture(t, 0x35, passthroughCodec{})
	targetFixture := newRevalidationFixture(t, 0x36, passthroughCodec{})
	relation := valueRelation(
		t,
		storedFixture,
		"assertion:old-codec-coordinate",
		storedFixture.signature,
		1,
	)
	observation := committedObservation(
		t,
		storedFixture.target.Ref(),
		"qnt_1234abcd",
		[]typedmemory.RelationInstance{relation},
	)

	report, err := Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: targetFixture.target,
		TargetRuntime: targetFixture.runtime,
	})
	if err != nil {
		t.Fatalf("Revalidate() error = %v", err)
	}
	if report.Posture() != typedmemory.RevalidationUnderdetermined {
		t.Fatalf(
			"Posture() = %s, want underdetermined",
			report.Posture().String(),
		)
	}
	outcome := report.Outcomes()[0]
	if outcome.Kind() != AssertionUnderdetermined ||
		!hasGroundCode(outcome, CodeValueMigrationRequired) {
		t.Fatalf(
			"outcome = %s, grounds = %#v",
			outcome.Kind().String(),
			outcome.Grounds(),
		)
	}
}

func TestRevalidateRequiresAppendOnlyMigrationWhenShapeCoordinateChanges(
	t *testing.T,
) {
	t.Parallel()
	storedFixture := newRevalidationFixture(t, 0x37, passthroughCodec{})
	targetFixture := newRevalidationFixtureWithCoordinates(
		t,
		0x38,
		0x37,
		"Haft.TextV2",
		typedmemory.ScalarText,
		passthroughCodec{},
	)
	relation := valueRelation(
		t,
		storedFixture,
		"assertion:old-shape-coordinate",
		storedFixture.signature,
		1,
	)
	observation := committedObservation(
		t,
		storedFixture.target.Ref(),
		"qnt_1234abcd",
		[]typedmemory.RelationInstance{relation},
	)

	report, err := Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: targetFixture.target,
		TargetRuntime: targetFixture.runtime,
	})
	if err != nil {
		t.Fatalf("Revalidate() error = %v", err)
	}
	if report.Posture() != typedmemory.RevalidationUnderdetermined ||
		report.Outcomes()[0].Kind() != AssertionUnderdetermined ||
		!hasGroundCode(
			report.Outcomes()[0],
			CodeValueMigrationRequired,
		) {
		t.Fatalf(
			"shape-transition report = %s/%s grounds=%#v",
			report.Posture().String(),
			report.Outcomes()[0].Kind().String(),
			report.Outcomes()[0].Grounds(),
		)
	}
}

func TestRevalidateMixedInvalidAndUnderdeterminedKeepsFullVector(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRevalidationFixture(t, 0x41, passthroughCodec{})
	invalid := valueRelation(
		t,
		fixture,
		"assertion:z-invalid",
		fixture.signature,
		2,
	)
	unknownID := mustTestValue(typedmemory.NewSignatureID("Haft.Unknown"))
	unknownSignature := mustTestValue(typedmemory.NewRelationSignatureRef(
		fixture.target.Ref(),
		unknownID,
	))
	under := valueRelation(
		t,
		fixture,
		"assertion:a-under",
		unknownSignature,
		1,
	)
	observation := committedObservation(
		t,
		fixture.target.Ref(),
		"qnt_1234abcd",
		[]typedmemory.RelationInstance{invalid, under},
	)
	report, err := Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: fixture.target,
		TargetRuntime: fixture.runtime,
	})
	if err != nil {
		t.Fatalf("Revalidate() error = %v", err)
	}
	if report.Posture() != typedmemory.RevalidationConflict {
		t.Fatalf("Posture() = %s, want conflict", report.Posture().String())
	}
	outcomes := report.Outcomes()
	if len(outcomes) != 2 ||
		outcomes[0].AssertionID().String() != "assertion:a-under" ||
		outcomes[0].Kind() != AssertionUnderdetermined ||
		outcomes[1].Kind() != AssertionInvalid {
		t.Fatalf("canonical full outcome vector = %#v", outcomeKeys(outcomes))
	}
	if len(report.AffectedAssertions()) != 2 {
		t.Fatal("mixed report lost a non-valid assertion")
	}
	decoded, err := DecodeCanonicalReport(report.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeCanonicalReport() error = %v", err)
	}
	if decoded.Digest() != report.Digest() ||
		len(decoded.Outcomes()) != 2 {
		t.Fatal("mixed outcome vector did not survive canonical round-trip")
	}
}

func TestReportDigestTracksTargetGraphRuntimeRelationAndGrounds(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRevalidationFixture(t, 0x51, passthroughCodec{})
	relation := valueRelation(
		t,
		fixture,
		"assertion:sensitive",
		fixture.signature,
		1,
	)
	observation := committedObservation(
		t,
		fixture.target.Ref(),
		"qnt_1234abcd",
		[]typedmemory.RelationInstance{relation},
	)
	baseline := mustTestValue(Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: fixture.target,
		TargetRuntime: fixture.runtime,
	}))
	otherProjectObservation := committedObservation(
		t,
		fixture.target.Ref(),
		"qnt_abcd1234",
		[]typedmemory.RelationInstance{relation},
	)
	otherGraph := mustTestValue(Revalidate(Input{
		CurrentGraph:  otherProjectObservation,
		TargetTypeEnv: fixture.target,
		TargetRuntime: fixture.runtime,
	}))
	otherRuntime := mustTestValue(Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: fixture.target,
		TargetRuntime: exactRuntime(
			t,
			fixture.codec,
			passthroughCodec{},
			"artifact:runtime-other",
		),
	}))
	otherTargetFixture := newRevalidationFixture(
		t,
		0x52,
		passthroughCodec{},
	)
	otherTarget := mustTestValue(Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: otherTargetFixture.target,
		TargetRuntime: otherTargetFixture.runtime,
	}))
	for name, digest := range map[string]typedmemory.SHA256Digest{
		"graph":   otherGraph.Digest(),
		"runtime": otherRuntime.Digest(),
		"target":  otherTarget.Digest(),
	} {
		if digest == baseline.Digest() {
			t.Fatalf("%s coordinate did not affect report digest", name)
		}
	}

	outcome := baseline.Outcomes()[0]
	differentGround := mustMissingGround(
		CodeTargetRelationFragmentUnavailable,
		"assertions.assertion:sensitive.signature",
		"synthetic sensitivity fixture",
		"target_relation_declaration_fragment",
		"other",
		"inspect-other",
	)
	changedOutcome := mustTestValue(newAssertionOutcome(
		outcome.AssertionID(),
		outcome.RelationDigest(),
		[]Ground{differentGround},
	))
	changedGroundReport := mustTestValue(newReport(
		baseline.TargetTypeEnv(),
		observation.GraphSnapshotBasis(),
		baseline.RuntimeBasisRef(),
		baseline.RuntimeCoordinateDigest(),
		[]AssertionOutcome{changedOutcome},
	))
	if changedGroundReport.Digest() == baseline.Digest() {
		t.Fatal("outcome grounds did not affect report digest")
	}
}

func TestReportCanonicalizationIgnoresOutcomeInputOrder(t *testing.T) {
	t.Parallel()
	fixture := newRevalidationFixture(t, 0x61, passthroughCodec{})
	first := valueRelation(
		t,
		fixture,
		"assertion:first",
		fixture.signature,
		1,
	)
	second := valueRelation(
		t,
		fixture,
		"assertion:second",
		fixture.signature,
		2,
	)
	observation := committedObservation(
		t,
		fixture.target.Ref(),
		"qnt_1234abcd",
		[]typedmemory.RelationInstance{first, second},
	)
	baseline := mustTestValue(Revalidate(Input{
		CurrentGraph:  observation,
		TargetTypeEnv: fixture.target,
		TargetRuntime: fixture.runtime,
	}))
	outcomes := baseline.Outcomes()
	reversed := []AssertionOutcome{outcomes[1], outcomes[0]}
	reordered := mustTestValue(newReport(
		baseline.TargetTypeEnv(),
		observation.GraphSnapshotBasis(),
		baseline.RuntimeBasisRef(),
		baseline.RuntimeCoordinateDigest(),
		reversed,
	))
	if reordered.Digest() != baseline.Digest() ||
		!bytes.Equal(reordered.CanonicalBytes(), baseline.CanonicalBytes()) {
		t.Fatal("permuting the complete outcome set changed the canonical report")
	}
}

func TestDecodeCanonicalReportRejectsTrailingBytes(t *testing.T) {
	t.Parallel()
	fixture := newRevalidationFixture(t, 0x62, passthroughCodec{})
	report := mustTestValue(Revalidate(Input{
		CurrentGraph: emptyObservation(
			t,
			fixture.target.Ref(),
			"qnt_1234abcd",
		),
		TargetTypeEnv: fixture.target,
		TargetRuntime: fixture.runtime,
	}))
	trailing := append(report.CanonicalBytes(), 0x01)
	if _, err := DecodeCanonicalReport(trailing); err == nil {
		t.Fatal("DecodeCanonicalReport() accepted trailing bytes")
	}
}

func newRevalidationFixture(
	t *testing.T,
	fill byte,
	codecImplementation typedmemory.CodecImplementation,
) revalidationFixture {
	t.Helper()
	return newRevalidationFixtureWithCoordinates(
		t,
		fill,
		fill,
		"Haft.BytesV1",
		typedmemory.ScalarBytes,
		codecImplementation,
	)
}

func newRevalidationFixtureWithCoordinates(
	t *testing.T,
	typeEnvFill byte,
	codecFill byte,
	shapeIDRaw string,
	scalarKind typedmemory.ScalarKind,
	codecImplementation typedmemory.CodecImplementation,
) revalidationFixture {
	t.Helper()
	ref := typeEnvRef(t, typeEnvFill)
	provenance, coverage := typeEnvProvenance(t, typeEnvFill)
	contextRef := mustTestValue(typedmemory.NewBoundedContextRef("ctx:test"))
	context := mustTestValue(typedmemory.NewBoundedContext(
		contextRef,
		provenance,
	))
	kindID := mustTestValue(typedmemory.NewKindID("Haft.Payload"))
	kind := mustTestValue(typedmemory.NewKindDefinition(kindID, provenance))
	valueKind := mustTestValue(typedmemory.NewValueKindRef(ref, kindID))
	shape := mustTestValue(typedmemory.NewScalarShape(scalarKind))
	shapeID := mustTestValue(typedmemory.NewShapeID(shapeIDRaw))
	shapeRef := mustTestValue(typedmemory.DeriveValueShapeRef(shapeID, shape))
	shapeDeclaration := mustTestValue(typedmemory.NewValueShapeDeclaration(
		shapeRef,
		shape,
		provenance,
	))
	codec := codecRef(t, codecFill)
	binding := mustTestValue(typedmemory.NewValueBinding(
		valueKind,
		shapeRef,
		codec,
		provenance,
	))
	slotID := mustTestValue(typedmemory.NewSlotKindID("Haft.PayloadSlot"))
	slotTarget := mustTestValue(typedmemory.NewValueSlotTarget(valueKind))
	slot := mustTestValue(typedmemory.NewSlotSpec(
		slotID,
		slotTarget,
		typedmemory.ExactlyOneCardinality(),
		provenance,
	))
	signatureID := mustTestValue(typedmemory.NewSignatureID("Haft.PayloadRelation"))
	signatureRef := mustTestValue(typedmemory.NewRelationSignatureRef(
		ref,
		signatureID,
	))
	signature := mustTestValue(typedmemory.NewRelationSignature(
		signatureRef,
		[]typedmemory.BoundedContextRef{contextRef},
		[]typedmemory.SlotSpec{slot},
		provenance,
	))
	availability := kindAvailability(
		t,
		ref,
		contextRef,
		kindID,
		provenance,
		typeEnvFill,
	)
	target := mustTestValue(
		typedmemory.NewTypeEnvBuilder(ref).
			SetSourceRevision(mustTestValue(typedmemory.NewSourceRevision(
				"test-source-revision-" + strings.Repeat("a", int(typeEnvFill%3)+1),
			))).
			SetCompilerSchemaVersion(
				mustTestValue(typedmemory.NewCompilerSchemaVersion("test.v1")),
			).
			SetCoverageManifest(coverage).
			AddBoundedContext(context).
			AddKindDefinition(kind).
			AddContextKindAvailability(availability).
			AddRelationSignature(signature).
			AddValueShape(shapeDeclaration).
			AddValueBinding(binding).
			Build(),
	)
	return revalidationFixture{
		target: target,
		runtime: exactRuntime(
			t,
			codec,
			codecImplementation,
			"artifact:runtime-"+strings.Repeat("a", int(typeEnvFill%7)+1),
		),
		codec:     codec,
		context:   contextRef,
		signature: signatureRef,
		slot:      slotID,
		valueKind: valueKind,
		shape:     shapeRef,
	}
}

func typeEnvWithContextOnly(
	t *testing.T,
	ref typedmemory.TypeEnvRef,
	contextRef typedmemory.BoundedContextRef,
	fill byte,
) typedmemory.TypeEnv {
	t.Helper()
	provenance, coverage := typeEnvProvenance(t, fill)
	context := mustTestValue(typedmemory.NewBoundedContext(
		contextRef,
		provenance,
	))
	return mustTestValue(
		typedmemory.NewTypeEnvBuilder(ref).
			SetSourceRevision(
				mustTestValue(typedmemory.NewSourceRevision("test-source-context-only")),
			).
			SetCompilerSchemaVersion(
				mustTestValue(typedmemory.NewCompilerSchemaVersion("test.v1")),
			).
			SetCoverageManifest(coverage).
			AddBoundedContext(context).
			Build(),
	)
}

func exactRuntime(
	t *testing.T,
	codec typedmemory.CodecRef,
	implementation typedmemory.CodecImplementation,
	artifactRaw string,
) projecttypeenvruntime.ExactTargetRuntimeRegistry {
	t.Helper()
	entry := mustTestValue(runtimemechanism.NewCodecCanonicalizationEntry(codec))
	artifact := mustTestValue(typedmemory.NewCarrierRef(artifactRaw))
	edition := mustTestValue(typedmemory.NewCarrierEdition("1.0.0"))
	catalog := mustTestValue(runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifact,
		edition,
		[]runtimemechanism.RuntimeMechanismEntryV1{entry},
	))
	mechanism := mustTestValue(
		projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog),
	)
	pin := mustTestValue(projecttypeenv.NewCodecRuntimeMechanismPin(
		projecttypeenv.CodecRuntimeMechanismPinInput{
			Codec:            codec,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	))
	basis := mustTestValue(projecttypeenv.SealRuntimeEvaluationBasis(
		[]projecttypeenv.RuntimeEvaluationMechanismPin{pin},
		catalog,
	))
	codecs := mustTestValue(
		typedmemory.NewCodecRegistry().Register(codec, implementation),
	)
	evaluators := mustTestValue(memberofruntime.NewRegistry(nil))
	resolution := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             codecs,
				MemberOfEvaluators: evaluators,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
					catalog,
				},
			},
		},
	)
	matched, ok := resolution.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("runtime resolution = %s, want matched", resolution.Kind().String())
	}
	registry, ok := matched.Registry()
	if !ok {
		t.Fatal("matched runtime did not expose registry")
	}
	return registry
}

func valueRelation(
	t *testing.T,
	fixture revalidationFixture,
	assertionRaw string,
	signature typedmemory.RelationSignatureRef,
	fillerCount int,
) typedmemory.RelationInstance {
	t.Helper()
	slice := contextSlice(t, fixture.context)
	valueBytes := []byte("payload")
	valueDigest := mustTestValue(typedmemory.ComputeTypedValueDigest(
		fixture.valueKind,
		fixture.shape,
		fixture.codec,
		valueBytes,
	))
	filler := typedMemoryEnvelope(
		"validated-by-value.v1",
		[]byte(fixture.valueKind.String()),
		[]byte(fixture.shape.String()),
		[]byte(fixture.codec.String()),
		valueBytes,
		[]byte(valueDigest.String()),
	)
	bindingFields := [][]byte{[]byte(fixture.slot.String())}
	for index := 0; index < fillerCount; index++ {
		bindingFields = append(bindingFields, filler)
	}
	binding := typedMemoryEnvelope(
		"validated-slot-binding.v1",
		bindingFields...,
	)
	raw := typedMemoryEnvelope(
		"validated-relation-instance.v2",
		[]byte(assertionRaw),
		[]byte(signature.String()),
		[]byte(slice.Ref().String()),
		slice.CanonicalBytes(),
		binding,
		[]byte("memory:test:revalidation"),
	)
	return mustTestValue(typedmemory.DecodeCanonicalRelationInstance(raw))
}

func contextSlice(
	t *testing.T,
	context typedmemory.BoundedContextRef,
) typedmemory.ContextSlice {
	t.Helper()
	gamma := mustTestValue(typedmemory.NewGammaPoint(
		time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC),
	))
	return mustTestValue(typedmemory.NewContextSlice(
		typedmemory.ContextSliceInput{
			Context:   context,
			GammaTime: gamma,
		},
	))
}

func emptyObservation(
	t *testing.T,
	activeTypeEnv typedmemory.TypeEnvRef,
	projectRaw string,
) projectgraphobservation.CurrentProjectGraphObservation {
	t.Helper()
	project := mustTestValue(projectidentity.ParseProjectID(projectRaw))
	revision := typedmemory.NewGraphRevision(0)
	basis := mustTestValue(projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: revision,
			Closure:       projecttypeenvselection.EmptyProjectGraphClosure{},
		},
	))
	active := mustTestValue(
		projectgraphobservation.NewCurrentActiveAssertionSet(
			project,
			revision,
			nil,
		),
	)
	return mustTestValue(
		projectgraphobservation.NewCurrentProjectGraphObservation(
			basis,
			activeTypeEnv,
			active,
		),
	)
}

func committedObservation(
	t *testing.T,
	activeTypeEnv typedmemory.TypeEnvRef,
	projectRaw string,
	relations []typedmemory.RelationInstance,
) projectgraphobservation.CurrentProjectGraphObservation {
	t.Helper()
	project := mustTestValue(projectidentity.ParseProjectID(projectRaw))
	revision := typedmemory.NewGraphRevision(1)
	event := mustTestValue(projecttypeenvselection.ParseGraphEventRef(
		"typed-memory-event:" + strings.Repeat("a", 64),
	))
	commit := mustTestValue(projecttypeenvselection.ParseGraphCommitRef(
		"typed-memory-commit:" + strings.Repeat("b", 64),
	))
	materialization := digest(t, 0x71)
	closure := mustTestValue(
		projecttypeenvselection.NewCommittedProjectGraphClosure(
			projecttypeenvselection.CommittedProjectGraphClosureInput{
				Event:                 event,
				Commit:                commit,
				MaterializationDigest: materialization,
			},
		),
	)
	basis := mustTestValue(projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: revision,
			Closure:       closure,
		},
	))
	assertions := make(
		[]projectgraphobservation.CurrentActiveAssertion,
		0,
		len(relations),
	)
	for index, relation := range relations {
		canonical := mustTestValue(relation.CanonicalBytes())
		relationDigest := mustTestValue(relation.Digest())
		assertions = append(
			assertions,
			mustTestValue(projectgraphobservation.NewCurrentActiveAssertion(
				projectgraphobservation.CurrentActiveAssertionInput{
					Relation:       relation,
					CanonicalBytes: canonical,
					Digest:         relationDigest,
					OriginEvent:    event,
					OriginRevision: revision,
					ChangeOrdinal:  uint64(index),
				},
			)),
		)
	}
	active := mustTestValue(
		projectgraphobservation.NewCurrentActiveAssertionSet(
			project,
			revision,
			assertions,
		),
	)
	return mustTestValue(
		projectgraphobservation.NewCurrentProjectGraphObservation(
			basis,
			activeTypeEnv,
			active,
		),
	)
}

func committedRelationalAssertionObservation(
	t *testing.T,
	activeTypeEnv typedmemory.TypeEnvRef,
	projectRaw string,
	assertion typedmemory.RelationalAssertion,
) projectgraphobservation.CurrentProjectGraphObservation {
	t.Helper()
	project := mustTestValue(projectidentity.ParseProjectID(projectRaw))
	revision := typedmemory.NewGraphRevision(1)
	event := mustTestValue(projecttypeenvselection.ParseGraphEventRef(
		"typed-memory-event:" + strings.Repeat("c", 64),
	))
	commit := mustTestValue(projecttypeenvselection.ParseGraphCommitRef(
		"typed-memory-commit:" + strings.Repeat("d", 64),
	))
	closure := mustTestValue(
		projecttypeenvselection.NewCommittedProjectGraphClosure(
			projecttypeenvselection.CommittedProjectGraphClosureInput{
				Event:                 event,
				Commit:                commit,
				MaterializationDigest: digest(t, 0x72),
			},
		),
	)
	basis := mustTestValue(projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: revision,
			Closure:       closure,
		},
	))
	canonical := mustTestValue(assertion.CanonicalBytes())
	assertionDigest := mustTestValue(assertion.Digest())
	current := mustTestValue(
		projectgraphobservation.NewCurrentActiveRelationalAssertion(
			projectgraphobservation.CurrentActiveRelationalAssertionInput{
				Assertion:      assertion,
				CanonicalBytes: canonical,
				Digest:         assertionDigest,
				OriginEvent:    event,
				OriginRevision: revision,
				ChangeOrdinal:  0,
			},
		),
	)
	active := mustTestValue(
		projectgraphobservation.NewCurrentActiveAssertionSet(
			project,
			revision,
			[]projectgraphobservation.CurrentActiveAssertion{current},
		),
	)
	return mustTestValue(
		projectgraphobservation.NewCurrentProjectGraphObservation(
			basis,
			activeTypeEnv,
			active,
		),
	)
}

func relationalAssertionFromRelation(
	t *testing.T,
	relation typedmemory.RelationInstance,
	modality typedmemory.AssertionModalityKind,
) typedmemory.RelationalAssertion {
	t.Helper()
	fields := [][]byte{
		[]byte(relation.Assertion().String()),
		[]byte(relation.Signature().String()),
		[]byte(relation.Slice().Ref().String()),
		relation.Slice().CanonicalBytes(),
		[]byte(modality.String()),
	}
	for _, binding := range relation.Bindings() {
		fields = append(fields, binding.CanonicalBytes())
	}
	fields = append(fields, []byte(relation.Provenance().String()))
	raw := typedMemoryEnvelope(
		"validated-relational-assertion.v3",
		fields...,
	)
	return mustTestValue(typedmemory.DecodeCanonicalRelationalAssertion(raw))
}

func typeEnvProvenance(
	t *testing.T,
	fill byte,
) (typedmemory.FPFSourceProvenance, typedmemory.CoverageManifest) {
	t.Helper()
	unit := mustTestValue(typedmemory.NewSourceUnitID(
		"spec:pattern_body:test-" + strings.Repeat("a", int(fill%9)+1),
	))
	revision := mustTestValue(typedmemory.NewSourceRevision(
		"test-source-" + strings.Repeat("b", int(fill%9)+1),
	))
	location := mustTestValue(typedmemory.NewPatternedSourceLocation(
		unit,
		revision,
		digest(t, fill),
		mustTestValue(typedmemory.NewSourceLineRange(1, 10)),
		mustTestValue(typedmemory.NewPatternID("A.6.5")),
	))
	provenance := mustTestValue(typedmemory.NewFPFSourceProvenance(
		mustTestValue(typedmemory.NewProvenanceRef(
			"prov:test:"+strings.Repeat("c", int(fill%9)+1),
		)),
		location,
		mustTestValue(typedmemory.NewCompilerRuleID("test.slot.v1")),
	))
	subject := mustTestValue(typedmemory.SourceUnitCoverage(unit))
	entry := mustTestValue(typedmemory.NewCompiledCoverageEntry(
		subject,
		location,
	))
	return provenance, mustTestValue(typedmemory.NewCoverageManifest(
		[]typedmemory.CoverageEntry{entry},
	))
}

func kindAvailability(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	context typedmemory.BoundedContextRef,
	kind typedmemory.KindID,
	provenance typedmemory.DeclarationProvenance,
	fill byte,
) typedmemory.ContextKindAvailability {
	t.Helper()
	symbol := mustTestValue(typedmemory.KindSymbolRef(kind))
	manifest := mustTestValue(typedmemory.NewSignatureManifestRef(
		"test.kind-availability",
		"1.0.0",
	))
	basis := mustTestValue(typedmemory.NewManifestSymbolBasis(
		manifest,
		typedmemory.ManifestProvide,
		symbol,
	))
	projectProvenance := mustTestValue(
		typedmemory.NewProjectSourceProvenanceBuilder(
			mustTestValue(typedmemory.NewProvenanceRef(
				"prov:test:kind-availability",
			)),
			mustTestValue(typedmemory.NewCarrierRef(
				"carrier:test:kind-availability",
			)),
			mustTestValue(typedmemory.NewCarrierEdition("1.0.0")),
			digest(t, fill+1),
		).
			SetDeclarationRange(
				mustTestValue(typedmemory.NewSourceLineRange(1, 1)),
			).
			SetCompilerRule(
				mustTestValue(typedmemory.NewCompilerRuleID(
					"test.kind-availability.v1",
				)),
			).
			SetBoundedContext(context).
			SetBaseTypeEnv(typeEnv).
			SetSignatureBlockRow(typedmemory.VocabularyRow).
			SetManifestBasis(basis).
			Build(),
	)
	contextSource := mustTestValue(
		typedmemory.NewContextKindAvailabilitySource(
			context.String(),
			projectProvenance,
		),
	)
	declarationSource := mustTestValue(
		typedmemory.NewContextKindAvailabilitySource(
			kind.String(),
			projectProvenance,
		),
	)
	extensionRef := mustTestValue(typedmemory.ParseTypeEnvExtensionRef(
		"typeenv-extension:" + manifest.ID() + "@" + digest(t, fill+2).String(),
	))
	provider := mustTestValue(
		typedmemory.NewExtensionKindAvailabilityProvider(
			typedmemory.ExtensionKindAvailabilityProviderInput{
				ExtensionRef:      extensionRef,
				Context:           context,
				ContextSource:     contextSource,
				Symbol:            symbol,
				DeclarationSource: declarationSource,
			},
		),
	)
	ground := mustTestValue(typedmemory.NewLocalContextKindAvailabilityGround(
		typedmemory.LocalContextKindAvailabilityGroundInput{
			Context:             context,
			KindID:              kind,
			ContextSource:       contextSource,
			ApplicabilitySource: contextSource,
			Provider:            provider,
		},
	))
	grounds := mustTestValue(typedmemory.NewContextKindAvailabilityGroundSet(
		[]typedmemory.ContextKindAvailabilityGround{ground},
	))
	return mustTestValue(typedmemory.NewContextKindAvailability(
		context,
		kind,
		grounds,
	))
}

func codecRef(t *testing.T, fill byte) typedmemory.CodecRef {
	t.Helper()
	return mustTestValue(typedmemory.NewCodecRef(
		mustTestValue(typedmemory.NewCodecID("Haft.Codec.PayloadV1")),
		mustTestValue(typedmemory.NewCanonicalizationVersion("v1")),
		digest(t, fill+3),
	))
}

func typeEnvRef(t *testing.T, fill byte) typedmemory.TypeEnvRef {
	t.Helper()
	return mustTestValue(typedmemory.NewTypeEnvRef(digest(t, fill)))
}

func digest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	const alphabet = "0123456789abcdef"
	raw := make([]byte, len("sha256:")+64)
	copy(raw, "sha256:")
	for index := len("sha256:"); index < len(raw); index++ {
		raw[index] = alphabet[int(fill+byte(index))%len(alphabet)]
	}
	return mustTestValue(typedmemory.NewSHA256Digest(string(raw)))
}

func typedMemoryEnvelope(domain string, fields ...[]byte) []byte {
	buffer := bytes.Buffer{}
	appendTestField(&buffer, []byte("haft.typedmemory.canonical-envelope.v1"))
	appendTestField(&buffer, []byte(domain))
	for _, field := range fields {
		appendTestField(&buffer, field)
	}
	return buffer.Bytes()
}

func appendTestField(buffer *bytes.Buffer, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	buffer.Write(length[:])
	buffer.Write(value)
}

func hasGroundCode(outcome AssertionOutcome, code GroundCode) bool {
	for _, ground := range outcome.Grounds() {
		if ground.Code() == code {
			return true
		}
	}
	return false
}

func outcomeKeys(outcomes []AssertionOutcome) []string {
	result := make([]string, 0, len(outcomes))
	for _, outcome := range outcomes {
		result = append(result, strings.Join([]string{
			outcome.AssertionID().String(),
			outcome.RelationDigest().String(),
			outcome.Kind().String(),
		}, ":"))
	}
	return result
}

func mustTestValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
