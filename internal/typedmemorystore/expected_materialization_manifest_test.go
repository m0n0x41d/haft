package typedmemorystore

import (
	"bytes"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func sameExpectedMaterializationManifest(
	left expectedMaterializationManifest,
	right expectedMaterializationManifest,
) bool {
	return left.digest == right.digest &&
		bytes.Equal(left.canonicalBytes, right.canonicalBytes)
}

func TestExpectedMaterializationManifestProjectsSnapshotOnlyDeclaration(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "manifest-declaration", "Manifest declaration")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"manifest-declaration",
		candidate,
	)
	request.admissionBatch = sealGenericDeclaration(t, fixture, candidate, 0)
	prepared, err := prepareGenericAdmission(request)
	if err != nil {
		t.Fatalf("prepareGenericAdmission: %v", err)
	}

	manifest, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		t.Fatalf("buildExpectedMaterializationManifest: %v", err)
	}
	if len(manifest.declarations) != 1 {
		t.Fatalf("declarations = %d; want 1", len(manifest.declarations))
	}
	if len(manifest.resolutions) != 0 ||
		len(manifest.evaluations) != 0 ||
		len(manifest.observableInputs) != 0 ||
		len(manifest.memberUses) != 0 ||
		len(manifest.orderedPrefixes) != 0 {
		t.Fatalf("snapshot-only declaration projected membership rows: %#v", manifest)
	}
	declaration := manifest.declarations[0]
	if declaration.changeOrdinal != 0 ||
		declaration.batchLocalRef == "" ||
		declaration.declarationDigest.String() == "" ||
		len(declaration.declarationBytes) == 0 {
		t.Fatalf("declaration coordinate is incomplete: %#v", declaration)
	}
	rebuilt, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		t.Fatalf("rebuild expected manifest: %v", err)
	}
	if !sameExpectedMaterializationManifest(manifest, rebuilt) {
		t.Fatal("same sealed admission produced a different expected manifest")
	}
}

func TestExpectedMaterializationManifestProjectsExactProspectiveBasis(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "manifest-exact-basis")
	prepared, err := prepareGenericAdmission(request)
	if err != nil {
		t.Fatalf("prepareGenericAdmission: %v", err)
	}

	manifest, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		t.Fatalf("buildExpectedMaterializationManifest: %v", err)
	}
	if len(manifest.declarations) != 1 {
		t.Fatalf("declarations = %d; want 1", len(manifest.declarations))
	}
	if len(manifest.resolutions) != 1 {
		t.Fatalf("resolutions = %d; want 1", len(manifest.resolutions))
	}
	if len(manifest.evaluations) != 1 {
		t.Fatalf("evaluations = %d; want 1", len(manifest.evaluations))
	}
	if len(manifest.observableInputs) != 2 {
		t.Fatalf("observable inputs = %d; want 2", len(manifest.observableInputs))
	}
	if len(manifest.memberUses) != 1 {
		t.Fatalf("member uses = %d; want 1", len(manifest.memberUses))
	}
	if len(manifest.orderedPrefixes) != 1 {
		t.Fatalf("ordered prefixes = %d; want 1", len(manifest.orderedPrefixes))
	}

	declaration := manifest.declarations[0]
	resolution := manifest.resolutions[0]
	evaluation := manifest.evaluations[0]
	prefix := manifest.orderedPrefixes[0]
	if resolution.resolutionKind != "same_batch_declaration" ||
		resolution.declarationChangeOrdinal != "0" ||
		resolution.batchLocalRef != declaration.batchLocalRef ||
		resolution.declarationDigest != declaration.declarationDigest.String() ||
		resolution.orderedCandidatePrefixDigest != prefix.prefixDigest.String() {
		t.Fatalf("same-batch resolution witness = %#v", resolution)
	}
	if prefix.endOrdinal != 1 {
		t.Fatalf("prefix end = %d; want relation ordinal 1", prefix.endOrdinal)
	}
	if evaluation.evaluationViewKind != "prospective_batch" ||
		evaluation.viewDeclarationChangeOrdinal != "0" ||
		evaluation.viewBatchLocalRef != declaration.batchLocalRef ||
		evaluation.viewDeclarationDigest != declaration.declarationDigest.String() ||
		evaluation.viewPrefixEndOrdinal != "1" ||
		evaluation.viewOrderedCandidatePrefixDigest != prefix.prefixDigest.String() {
		t.Fatalf("prospective evaluation witness = %#v", evaluation)
	}
	inputSetDigest, err := typedmemory.ComputeMemberOfObservableInputSetDigest(
		fixture.observableInputs,
	)
	if err != nil {
		t.Fatalf("ComputeMemberOfObservableInputSetDigest: %v", err)
	}
	if evaluation.observableInputCount != 2 ||
		evaluation.observableInputSetDigest != inputSetDigest {
		t.Fatalf("observable set projection = count %d digest %s", evaluation.observableInputCount, evaluation.observableInputSetDigest.String())
	}
	inputs := manifest.observableInputs
	if inputs[0].evaluationRef != evaluation.evaluationRef ||
		inputs[0].inputOrdinal != 0 ||
		inputs[0].observableInputRef != "observable:exact-basis/a" ||
		inputs[1].inputOrdinal != 1 ||
		inputs[1].observableInputRef != "observable:exact-basis/b" {
		t.Fatalf("ordered observable tuples = %#v", inputs)
	}
	use := manifest.memberUses[0]
	if use.filler.changeOrdinal != 1 ||
		use.filler.slotOrdinal != 0 ||
		use.filler.fillerOrdinal != 0 ||
		use.queriedValueKindRef != fixture.entityKind.String() ||
		use.evaluationRef != evaluation.evaluationRef {
		t.Fatalf("MemberOf use coordinate = %#v", use)
	}
	assertManifestHasRowKind(t, manifest, "entity_declaration")
	assertManifestHasRowKind(t, manifest, relationalAssertionStorageFamily.resolutionRowKind)
	assertManifestHasRowKind(t, manifest, "memberof_evaluation")
	assertManifestHasRowKind(t, manifest, "memberof_observable_input")
	assertManifestHasRowKind(t, manifest, relationalAssertionStorageFamily.memberOfUseRowKind)
	assertManifestHasRowKind(t, manifest, "ordered_candidate_prefix")
}

func TestExpectedMaterializationManifestStrictDecodeRoundTripsAndRejectsExtension(
	t *testing.T,
) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "manifest-strict-decode")
	prepared, err := prepareGenericAdmission(request)
	if err != nil {
		t.Fatalf("prepareGenericAdmission: %v", err)
	}
	manifest, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		t.Fatalf("buildExpectedMaterializationManifest: %v", err)
	}
	decoded, err := decodeExpectedMaterializationManifest(
		manifest.CanonicalBytes(),
		manifest.Digest(),
		manifest.basisRevision,
	)
	if err != nil {
		t.Fatalf("decodeExpectedMaterializationManifest: %v", err)
	}
	if !sameExpectedMaterializationManifest(manifest, decoded) {
		t.Fatal("strictly decoded expected-materialization manifest changed bytes or digest")
	}
	fields, err := decodeCanonicalStorageFields(
		manifest.CanonicalBytes(),
		"typed-memory-expected-materialization-manifest.v1",
	)
	if err != nil {
		t.Fatalf("decode valid manifest fields: %v", err)
	}
	fields = append(fields, "unknown-extension")
	extended := canonicalStorageFields(
		"typed-memory-expected-materialization-manifest.v1",
		fields,
	)
	extendedDigest, err := digestBytes(extended)
	if err != nil {
		t.Fatalf("digest extended manifest: %v", err)
	}
	_, err = decodeExpectedMaterializationManifest(
		extended,
		extendedDigest,
		manifest.basisRevision,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("extended manifest error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	fields = append([]string(nil), fields[:len(fields)-1]...)
	if len(fields) < 5 || fields[3] != "declarations" {
		t.Fatalf("unexpected valid manifest field prefix: %#v", fields)
	}
	fields[4] = "2147483647"
	oversized := canonicalStorageFields(
		"typed-memory-expected-materialization-manifest.v1",
		fields,
	)
	oversizedDigest, err := digestBytes(oversized)
	if err != nil {
		t.Fatalf("digest oversized manifest: %v", err)
	}
	_, err = decodeExpectedMaterializationManifest(
		oversized,
		oversizedDigest,
		manifest.basisRevision,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("oversized manifest count error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestExpectedMaterializationWitnessesAreCoordinateSensitive(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "manifest-coordinate-sensitivity")
	prepared, err := prepareGenericAdmission(request)
	if err != nil {
		t.Fatalf("prepareGenericAdmission: %v", err)
	}
	manifest, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		t.Fatalf("buildExpectedMaterializationManifest: %v", err)
	}

	input := fixture.observableInputs[0]
	originalTuple := newExpectedObservableInputTuple(
		manifest.evaluations[0].evaluationRef,
		0,
		input,
	)
	shiftedTuple := newExpectedObservableInputTuple(
		manifest.evaluations[0].evaluationRef,
		100,
		input,
	)
	if bytes.Equal(originalTuple.canonicalBytes, shiftedTuple.canonicalBytes) {
		t.Fatal("observable input ordinal is absent from the exact tuple witness")
	}

	evaluation := manifest.evaluations[0]
	originalEvaluation := canonicalEvaluationWitness(evaluation)
	evaluation.viewBatchLocalRef = "local:wrong-same-batch-witness"
	if bytes.Equal(originalEvaluation, canonicalEvaluationWitness(evaluation)) {
		t.Fatal("prospective view batch-local ref is absent from the evaluation witness")
	}

	resolution := manifest.resolutions[0]
	originalResolution := canonicalResolutionWitness(resolution)
	resolution.localReferenceKindRef = "typeenv:wrong/refkind:wrong"
	if bytes.Equal(originalResolution, canonicalResolutionWitness(resolution)) {
		t.Fatal("same-batch local RefKind is absent from the resolution witness")
	}

	use := manifest.memberUses[0]
	originalUse := canonicalMemberUseCoordinate(use)
	use.queriedValueKindRef = "typeenv:wrong/kind:wrong"
	if bytes.Equal(originalUse, canonicalMemberUseCoordinate(use)) {
		t.Fatal("queried ValueKind is absent from the MemberOf-use coordinate")
	}
}

func assertManifestHasRowKind(
	t *testing.T,
	manifest expectedMaterializationManifest,
	want string,
) {
	t.Helper()
	for _, row := range manifest.semanticRows {
		if row.rowKind == want {
			return
		}
	}
	t.Fatalf("expected manifest lacks semantic row kind %q", want)
}
