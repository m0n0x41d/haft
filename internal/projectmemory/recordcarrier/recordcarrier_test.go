package recordcarrier

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestProjectRecordCarrierClosedVariantsRoundTrip(t *testing.T) {
	t.Parallel()

	variants := []ProjectRecordCarrierVariantV1{
		GenericProjectRecordVariantV1{},
		DecisionRecordVariantV1{},
		SpecSectionRecordVariantV1{},
		EvidenceRecordVariantV1{},
		SupportingEpistemeRecordVariantV1{},
		WorkRecordVariantV1{},
		WorkPlanRecordVariantV1{},
	}
	for _, variant := range variants {
		variant := variant
		t.Run(variant.Token(), func(t *testing.T) {
			t.Parallel()

			carrier := testCarrier(t, variant)
			decoded, err := DecodeProjectRecordCarrierV1(carrier.CanonicalBytes())
			if err != nil {
				t.Fatalf("DecodeProjectRecordCarrierV1() error = %v", err)
			}
			if decoded.Ref() != carrier.Ref() ||
				decoded.Edition() != carrier.Edition() ||
				decoded.Digest() != carrier.Digest() ||
				decoded.EntityID() != carrier.EntityID() ||
				decoded.BoundedContext() != carrier.BoundedContext() ||
				decoded.Variant().Token() != variant.Token() {
				t.Fatal("project-record carrier round-trip changed exact identity")
			}
			verified, err := VerifyProjectRecordCarrierV1(
				carrier.Ref(),
				carrier.Edition(),
				carrier.Digest(),
				carrier.CanonicalBytes(),
			)
			if err != nil || verified.Ref() != carrier.Ref() {
				t.Fatalf("VerifyProjectRecordCarrierV1() = %#v, %v", verified, err)
			}
		})
	}
}

func TestProjectRecordCarrierIdentityIsContentAddressed(t *testing.T) {
	t.Parallel()

	decision := testCarrier(t, DecisionRecordVariantV1{})
	work := testCarrier(t, WorkRecordVariantV1{})
	otherEntity, err := SealProjectRecordCarrierV1(
		testEntity(t, "entity:project-record-2"),
		testContext(t, "context:haft-project"),
		DecisionRecordVariantV1{},
	)
	if err != nil {
		t.Fatalf("SealProjectRecordCarrierV1() error = %v", err)
	}
	if decision.Digest() == work.Digest() || decision.Ref() == work.Ref() {
		t.Fatal("changing the closed carrier variant did not change content identity")
	}
	if decision.Digest() == otherEntity.Digest() || decision.Ref() == otherEntity.Ref() {
		t.Fatal("changing the record EntityID did not change content identity")
	}
	forgedDigest := testDigest(t, 0xfe)
	forgedRef, err := typedmemory.NewCarrierRef("project-record-carrier:" + forgedDigest.String())
	if err != nil {
		t.Fatalf("NewCarrierRef() error = %v", err)
	}
	if _, err := VerifyProjectRecordCarrierV1(
		forgedRef,
		decision.Edition(),
		forgedDigest,
		decision.CanonicalBytes(),
	); err == nil {
		t.Fatal("forged carrier reference/digest was accepted")
	}
}

func TestProjectRecordCarrierCodecRejectsUnknownTrailingNoncanonicalAndInvalidUTF8(t *testing.T) {
	t.Parallel()

	carrier := testCarrier(t, GenericProjectRecordVariantV1{})
	canonical := carrier.CanonicalBytes()

	unknown := append([]byte(nil), canonical[:len(canonical)-1]...)
	unknown = append(unknown, []byte(`,"unknown":true}`)...)
	if _, err := DecodeProjectRecordCarrierV1(unknown); err == nil {
		t.Fatal("unknown carrier field was accepted")
	}

	trailing := append(append([]byte(nil), canonical...), []byte(`{}`)...)
	if _, err := DecodeProjectRecordCarrierV1(trailing); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing carrier error = %v", err)
	}

	noncanonical := append([]byte(" "), canonical...)
	if _, err := DecodeProjectRecordCarrierV1(noncanonical); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("noncanonical carrier error = %v", err)
	}

	invalidUTF8 := append([]byte(nil), canonical...)
	needle := []byte("project_record")
	index := bytes.Index(invalidUTF8, needle)
	if index < 0 {
		t.Fatalf("carrier does not contain %q", needle)
	}
	invalidUTF8[index] = 0xff
	if _, err := DecodeProjectRecordCarrierV1(invalidUTF8); err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
		t.Fatalf("invalid UTF-8 carrier error = %v", err)
	}

	unsupportedVariant := decodeCarrierDTO(t, canonical)
	unsupportedVariant.Variant = "problem_card"
	if _, err := DecodeProjectRecordCarrierV1(marshalCanonicalTest(t, unsupportedVariant)); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported carrier variant error = %v", err)
	}
}

func TestEntityRecordCarrierBindingAndSourceRoundTrip(t *testing.T) {
	t.Parallel()

	fixture := testRecordSourceFixture(t, DecisionRecordVariantV1{})
	binding := fixture.binding
	if binding.ProjectID() != fixture.project ||
		binding.EntityID() != fixture.entity ||
		binding.BoundedContext() != fixture.context ||
		binding.RecordVariant().Token() != (DecisionRecordVariantV1{}).Token() ||
		binding.CarrierRef() != fixture.carrier.Ref() ||
		binding.CarrierEdition() != fixture.carrier.Edition() ||
		binding.CarrierDigest() != fixture.carrier.Digest() ||
		binding.CarrierSchemaVersion() != fixture.carrier.SchemaVersion() ||
		binding.MappingManifestRef() != fixture.manifest ||
		binding.AdapterVersion() != fixture.adapter {
		t.Fatal("binding lost an exact project/carrier coordinate")
	}
	decodedBinding, err := DecodeEntityRecordCarrierBindingV1(binding.CanonicalBytes())
	if err != nil || decodedBinding.Ref() != binding.Ref() {
		t.Fatalf("DecodeEntityRecordCarrierBindingV1() = %#v, %v", decodedBinding, err)
	}
	verifiedBinding, err := VerifyEntityRecordCarrierBindingV1(
		binding.Ref(),
		binding.CanonicalBytes(),
	)
	if err != nil || verifiedBinding.Ref() != binding.Ref() {
		t.Fatalf("VerifyEntityRecordCarrierBindingV1() = %#v, %v", verifiedBinding, err)
	}

	source := fixture.source
	decodedSource, err := DecodeRecordMembershipSourceV1(source.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeRecordMembershipSourceV1() error = %v", err)
	}
	if decodedSource.Digest() != source.Digest() ||
		decodedSource.ProjectID() != fixture.project ||
		decodedSource.EntityID() != fixture.entity ||
		decodedSource.BoundedContext() != fixture.context ||
		decodedSource.RecordVariant().Token() != (DecisionRecordVariantV1{}).Token() {
		t.Fatal("record membership source round-trip changed exact basis")
	}
	observable := source.ObservableInput()
	if observable.Digest() != source.Digest() ||
		observable.Reference().String() != "record-membership-source:"+source.Digest().String() {
		t.Fatal("record membership source observable is not derived from exact source bytes")
	}
	if _, err := VerifyRecordMembershipSourceV1(observable, source.CanonicalBytes()); err != nil {
		t.Fatalf("VerifyRecordMembershipSourceV1() error = %v", err)
	}
}

func TestBindingAndSourceCodecsRejectUnknownNoncanonicalAndInvalidUTF8(t *testing.T) {
	t.Parallel()

	fixture := testRecordSourceFixture(t, WorkRecordVariantV1{})
	bindingCanonical := fixture.binding.CanonicalBytes()
	unknownBinding := append([]byte(nil), bindingCanonical[:len(bindingCanonical)-1]...)
	unknownBinding = append(unknownBinding, []byte(`,"unknown":true}`)...)
	if _, err := DecodeEntityRecordCarrierBindingV1(unknownBinding); err == nil {
		t.Fatal("unknown binding field was accepted")
	}
	noncanonicalBinding := append(bindingCanonical, '\n')
	if _, err := DecodeEntityRecordCarrierBindingV1(noncanonicalBinding); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("noncanonical binding error = %v", err)
	}
	invalidBindingUTF8 := append([]byte(nil), bindingCanonical...)
	index := bytes.Index(invalidBindingUTF8, []byte("artifact-adapter"))
	if index < 0 {
		t.Fatal("binding does not contain adapter version")
	}
	invalidBindingUTF8[index] = 0xff
	if _, err := DecodeEntityRecordCarrierBindingV1(invalidBindingUTF8); err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
		t.Fatalf("invalid UTF-8 binding error = %v", err)
	}

	sourceCanonical := fixture.source.CanonicalBytes()
	unknownSource := append([]byte(nil), sourceCanonical[:len(sourceCanonical)-1]...)
	unknownSource = append(unknownSource, []byte(`,"unknown":true}`)...)
	if _, err := DecodeRecordMembershipSourceV1(unknownSource); err == nil {
		t.Fatal("unknown source field was accepted")
	}
	noncanonicalSource := append([]byte(" "), sourceCanonical...)
	if _, err := DecodeRecordMembershipSourceV1(noncanonicalSource); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("noncanonical source error = %v", err)
	}
	invalidSourceUTF8 := append([]byte(nil), sourceCanonical...)
	index = bytes.Index(invalidSourceUTF8, []byte(RecordMembershipSourceSchemaVersionV1))
	if index < 0 {
		t.Fatal("source does not contain schema version")
	}
	invalidSourceUTF8[index] = 0xff
	if _, err := DecodeRecordMembershipSourceV1(invalidSourceUTF8); err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
		t.Fatalf("invalid UTF-8 source error = %v", err)
	}
}

func TestStrongCoordinatesRejectTrimmedOrFloatingRepresentations(t *testing.T) {
	t.Parallel()

	carrierDTO := decodeCarrierDTO(
		t,
		testCarrier(t, GenericProjectRecordVariantV1{}).CanonicalBytes(),
	)
	carrierDTO.EntityID = " " + carrierDTO.EntityID
	if _, err := DecodeProjectRecordCarrierV1(marshalCanonicalTest(t, carrierDTO)); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("trimmed carrier EntityID error = %v", err)
	}

	fixture := testRecordSourceFixture(t, GenericProjectRecordVariantV1{})
	bindingDTO := decodeBindingDTO(t, fixture.binding.CanonicalBytes())
	bindingDTO.CarrierEdition = " latest "
	if _, err := DecodeEntityRecordCarrierBindingV1(marshalCanonicalTest(t, bindingDTO)); err == nil {
		t.Fatal("floating/noncanonical carrier edition was accepted")
	}

	for _, floating := range []string{
		"now",
		"LATEST",
		"current",
		"implicit",
		"head",
		"*",
		"1.x",
		">=1",
		"1.0.0-01",
	} {
		if _, err := NewMappingManifestRef(
			"Haft.Adapter",
			floating,
			testDigest(t, 0x73),
		); err == nil {
			t.Fatalf("floating mapping-manifest version %q was accepted", floating)
		}
	}
	if _, err := NewMappingManifestRef(
		"Haft.Adapter",
		"1.0.0-rc.1+build.7",
		testDigest(t, 0x74),
	); err != nil {
		t.Fatalf("exact mapping-manifest version rejected: %v", err)
	}
}

func TestRecordMembershipSourceRejectsProjectEntityContextAndVariantSubstitution(t *testing.T) {
	t.Parallel()

	fixture := testRecordSourceFixture(t, DecisionRecordVariantV1{})
	tests := []struct {
		name    string
		project projectidentity.ProjectID
		entity  typedmemory.EntityID
		context typedmemory.BoundedContextRef
		binding EntityRecordCarrierBindingV1
		want    string
	}{
		{
			name:    "project",
			project: testProject(t, "qnt_01234567"),
			entity:  fixture.entity,
			context: fixture.context,
			binding: fixture.binding,
			want:    "project mismatch",
		},
		{
			name:    "entity lookup",
			project: fixture.project,
			entity:  testEntity(t, "entity:other-record"),
			context: fixture.context,
			binding: fixture.binding,
			want:    "entity lookup mismatch",
		},
		{
			name:    "context lookup",
			project: fixture.project,
			entity:  fixture.entity,
			context: testContext(t, "context:other-project"),
			binding: fixture.binding,
			want:    "context lookup mismatch",
		},
		{
			name:    "bound entity",
			project: fixture.project,
			entity:  fixture.entity,
			context: fixture.context,
			binding: mutateBinding(t, fixture.binding, func(value *entityRecordCarrierBindingCanonicalV1) {
				value.EntityID = "entity:other-record"
			}),
			want: "bound entity mismatch",
		},
		{
			name:    "bound context",
			project: fixture.project,
			entity:  fixture.entity,
			context: fixture.context,
			binding: mutateBinding(t, fixture.binding, func(value *entityRecordCarrierBindingCanonicalV1) {
				value.BoundedContext = "context:other-project"
			}),
			want: "bound context mismatch",
		},
		{
			name:    "variant",
			project: fixture.project,
			entity:  fixture.entity,
			context: fixture.context,
			binding: mutateBinding(t, fixture.binding, func(value *entityRecordCarrierBindingCanonicalV1) {
				value.RecordVariant = WorkRecordVariantV1{}.Token()
			}),
			want: "carrier variant mismatch",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRecordMembershipSourceV1(
				test.project,
				test.entity,
				test.context,
				fixture.carrier,
				test.binding,
			)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("source substitution error = %v; want %q", err, test.want)
			}
		})
	}
}

func TestRecordMembershipSourceRejectsCarrierLocatorCrossSubstitution(t *testing.T) {
	t.Parallel()

	fixture := testRecordSourceFixture(t, EvidenceRecordVariantV1{})
	otherCarrier, err := SealProjectRecordCarrierV1(
		fixture.entity,
		fixture.context,
		WorkPlanRecordVariantV1{},
	)
	if err != nil {
		t.Fatalf("SealProjectRecordCarrierV1(other) error = %v", err)
	}
	otherBinding, err := SealEntityRecordCarrierBindingV1(
		fixture.project,
		otherCarrier,
		fixture.manifest,
		fixture.adapter,
	)
	if err != nil {
		t.Fatalf("SealEntityRecordCarrierBindingV1(other) error = %v", err)
	}
	_, err = NewRecordMembershipSourceV1(
		fixture.project,
		fixture.entity,
		fixture.context,
		fixture.carrier,
		otherBinding,
	)
	if err == nil || !strings.Contains(err.Error(), "carrier reference mismatch") {
		t.Fatalf("coherent locator cross-substitution error = %v", err)
	}

	mutations := []struct {
		name   string
		mutate func(*entityRecordCarrierBindingCanonicalV1)
		want   string
	}{
		{
			name: "incoherent reference",
			mutate: func(value *entityRecordCarrierBindingCanonicalV1) {
				value.CarrierRef = "project-record-carrier:" + testDigest(t, 0x91).String()
			},
			want: "does not match its content digest",
		},
		{
			name: "unsupported edition",
			mutate: func(value *entityRecordCarrierBindingCanonicalV1) {
				value.CarrierEdition = "2"
			},
			want: "edition",
		},
		{
			name: "incoherent digest",
			mutate: func(value *entityRecordCarrierBindingCanonicalV1) {
				value.CarrierDigest = testDigest(t, 0x92).String()
			},
			want: "does not match its content digest",
		},
		{
			name: "unsupported schema",
			mutate: func(value *entityRecordCarrierBindingCanonicalV1) {
				value.CarrierSchema = "haft.project-record-carrier/v2"
			},
			want: "unsupported bound",
		},
	}
	for _, mutation := range mutations {
		mutation := mutation
		t.Run(mutation.name, func(t *testing.T) {
			t.Parallel()

			value := decodeBindingDTO(t, fixture.binding.CanonicalBytes())
			mutation.mutate(&value)
			_, err := DecodeEntityRecordCarrierBindingV1(
				marshalCanonicalTest(t, value),
			)
			if err == nil || !strings.Contains(err.Error(), mutation.want) {
				t.Fatalf("binding locator coherence error = %v; want %q", err, mutation.want)
			}
		})
	}
}

func TestRecordMembershipSourceRejectsNestedForgeryAndOuterVariantMismatch(t *testing.T) {
	t.Parallel()

	fixture := testRecordSourceFixture(t, SpecSectionRecordVariantV1{})
	encoded := decodeSourceDTO(t, fixture.source.CanonicalBytes())
	encoded.RecordVariant = WorkPlanRecordVariantV1{}.Token()
	if _, err := DecodeRecordMembershipSourceV1(
		marshalCanonicalTest(t, encoded),
	); err == nil || !strings.Contains(err.Error(), "outer variant mismatch") {
		t.Fatalf("outer variant substitution error = %v", err)
	}

	encoded = decodeSourceDTO(t, fixture.source.CanonicalBytes())
	encoded.CarrierBytes[0] ^= 0xff
	if _, err := DecodeRecordMembershipSourceV1(
		marshalCanonicalTest(t, encoded),
	); err == nil {
		t.Fatal("nested carrier byte mutation was accepted")
	}

	forgedObservable, err := typedmemory.NewMemberOfObservableInput(
		testObservableRef(t, "record-membership-source:"+testDigest(t, 0xa1).String()),
		testDigest(t, 0xa1),
	)
	if err != nil {
		t.Fatalf("NewMemberOfObservableInput() error = %v", err)
	}
	if _, err := VerifyRecordMembershipSourceV1(
		forgedObservable,
		fixture.source.CanonicalBytes(),
	); err == nil {
		t.Fatal("forged membership observable was accepted")
	}

	trailing := append(fixture.source.CanonicalBytes(), []byte(`{}`)...)
	if _, err := DecodeRecordMembershipSourceV1(trailing); err == nil {
		t.Fatal("trailing source bytes were accepted")
	}
}

func TestRecordCarrierArtifactsDefensivelyCopyBytes(t *testing.T) {
	t.Parallel()

	fixture := testRecordSourceFixture(t, SupportingEpistemeRecordVariantV1{})
	carrierCanonical := fixture.carrier.CanonicalBytes()
	bindingCanonical := fixture.binding.CanonicalBytes()
	sourceCanonical := fixture.source.CanonicalBytes()

	carrierCanonical[0] ^= 0xff
	bindingCanonical[0] ^= 0xff
	sourceCanonical[0] ^= 0xff
	if fixture.carrier.CanonicalBytes()[0] == carrierCanonical[0] {
		t.Fatal("carrier CanonicalBytes() leaked mutable state")
	}
	if fixture.binding.CanonicalBytes()[0] == bindingCanonical[0] {
		t.Fatal("binding CanonicalBytes() leaked mutable state")
	}
	if fixture.source.CanonicalBytes()[0] == sourceCanonical[0] {
		t.Fatal("source CanonicalBytes() leaked mutable state")
	}

	input := fixture.source.CanonicalBytes()
	decoded, err := DecodeRecordMembershipSourceV1(input)
	if err != nil {
		t.Fatalf("DecodeRecordMembershipSourceV1() error = %v", err)
	}
	input[0] ^= 0xff
	if !bytes.Equal(decoded.CanonicalBytes(), fixture.source.CanonicalBytes()) {
		t.Fatal("source retained caller-owned decode bytes")
	}
}

func TestRecordClassificationSourceIsCurrentAndByteDisjointFromMembership(
	t *testing.T,
) {
	t.Parallel()
	fixture := testRecordSourceFixture(t, DecisionRecordVariantV1{})
	current, err := SealRecordClassificationSourceV1(
		fixture.project,
		fixture.entity,
		fixture.context,
		fixture.carrier,
		fixture.binding,
	)
	if err != nil {
		t.Fatalf("SealRecordClassificationSourceV1() error = %v", err)
	}
	if current.SchemaVersion() != RecordClassificationSourceSchemaVersionV1 ||
		!strings.HasPrefix(current.Ref().String(), "record-classification-source:") {
		t.Fatal("current classification source lost its disjoint schema or reference domain")
	}
	if bytes.Equal(current.CanonicalBytes(), fixture.source.CanonicalBytes()) ||
		current.Ref().String() == fixture.source.ObservableInput().Reference().String() {
		t.Fatal("current classification source aliases historical membership bytes")
	}
	decoded, err := DecodeRecordClassificationSourceV1(current.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeRecordClassificationSourceV1() error = %v", err)
	}
	if decoded.Ref() != current.Ref() ||
		decoded.Digest() != current.Digest() ||
		decoded.RecordVariant().Token() != current.RecordVariant().Token() {
		t.Fatal("current classification source round-trip changed its exact coordinate")
	}
	if _, err := VerifyRecordClassificationSourceV1(
		current.Ref(),
		current.Digest(),
		current.CanonicalBytes(),
	); err != nil {
		t.Fatalf("VerifyRecordClassificationSourceV1() error = %v", err)
	}
	tampered := current.CanonicalBytes()
	tampered[len(tampered)-2] ^= 0x01
	if _, err := DecodeRecordClassificationSourceV1(tampered); err == nil {
		t.Fatal("tampered current classification source was accepted")
	}
	if _, err := DecodeRecordClassificationSourceV1(
		append(current.CanonicalBytes(), []byte(`{}`)...),
	); err == nil {
		t.Fatal("current classification source accepted trailing bytes")
	}
}

func TestRecordCarrierContractHasNoKindOrAuthoritySurface(t *testing.T) {
	t.Parallel()

	types := []reflect.Type{
		reflect.TypeOf(ProjectRecordCarrierV1{}),
		reflect.TypeOf(EntityRecordCarrierBindingV1{}),
		reflect.TypeOf(RecordMembershipSourceV1{}),
		reflect.TypeOf(RecordClassificationSourceV1{}),
	}
	for _, current := range types {
		for index := 0; index < current.NumField(); index++ {
			name := strings.ToLower(current.Field(index).Name)
			for _, forbidden := range []string{"kindid", "approval", "authority", "entityofconcern", "relation"} {
				if strings.Contains(name, forbidden) {
					t.Fatalf("%s exposes forbidden field %q", current.Name(), current.Field(index).Name)
				}
			}
		}
		for _, method := range []string{"Admit", "Activate", "Approve", "Authorize", "Member", "NotMember"} {
			if _, exists := current.MethodByName(method); exists {
				t.Fatalf("%s exposes forbidden method %q", current.Name(), method)
			}
		}
	}
}

func TestMappingManifestAndAdapterReferencesAreExact(t *testing.T) {
	t.Parallel()

	manifest := testManifest(t)
	parsed, err := ParseMappingManifestRef(manifest.String())
	if err != nil || parsed != manifest {
		t.Fatalf("ParseMappingManifestRef() = %#v, %v", parsed, err)
	}
	for _, floating := range []string{
		"now",
		"LATEST",
		"current",
		"implicit",
		"head",
		"*",
		"1.x",
		">=1",
		"artifact-adapter/latest",
		"artifact-adapter/1.0.0-01",
	} {
		if _, err := NewAdapterVersion(floating); err == nil {
			t.Fatalf("floating adapter version %q was accepted", floating)
		}
	}
	for _, exact := range []string{
		"1.0.0",
		"artifact-adapter/1.0.0",
		"artifact-adapter/1.2.3-rc.1+build.7",
	} {
		if _, err := NewAdapterVersion(exact); err != nil {
			t.Fatalf("exact adapter version %q rejected: %v", exact, err)
		}
	}
}

type recordSourceFixture struct {
	project  projectidentity.ProjectID
	entity   typedmemory.EntityID
	context  typedmemory.BoundedContextRef
	carrier  ProjectRecordCarrierV1
	manifest MappingManifestRef
	adapter  AdapterVersion
	binding  EntityRecordCarrierBindingV1
	source   RecordMembershipSourceV1
}

func testRecordSourceFixture(
	t *testing.T,
	variant ProjectRecordCarrierVariantV1,
) recordSourceFixture {
	t.Helper()
	project := testProject(t, "qnt_deadbeef")
	carrier := testCarrier(t, variant)
	manifest := testManifest(t)
	adapter, err := NewAdapterVersion("artifact-adapter/1.0.0")
	if err != nil {
		t.Fatalf("NewAdapterVersion() error = %v", err)
	}
	binding, err := SealEntityRecordCarrierBindingV1(project, carrier, manifest, adapter)
	if err != nil {
		t.Fatalf("SealEntityRecordCarrierBindingV1() error = %v", err)
	}
	source, err := NewRecordMembershipSourceV1(
		project,
		carrier.EntityID(),
		carrier.BoundedContext(),
		carrier,
		binding,
	)
	if err != nil {
		t.Fatalf("NewRecordMembershipSourceV1() error = %v", err)
	}
	return recordSourceFixture{
		project:  project,
		entity:   carrier.EntityID(),
		context:  carrier.BoundedContext(),
		carrier:  carrier,
		manifest: manifest,
		adapter:  adapter,
		binding:  binding,
		source:   source,
	}
}

func testCarrier(
	t *testing.T,
	variant ProjectRecordCarrierVariantV1,
) ProjectRecordCarrierV1 {
	t.Helper()
	carrier, err := SealProjectRecordCarrierV1(
		testEntity(t, "entity:project-record-1"),
		testContext(t, "context:haft-project"),
		variant,
	)
	if err != nil {
		t.Fatalf("SealProjectRecordCarrierV1() error = %v", err)
	}
	return carrier
}

func testManifest(t *testing.T) MappingManifestRef {
	t.Helper()
	manifest, err := NewMappingManifestRef(
		"Haft.DecisionRecordAdapter",
		"1.0.0",
		testDigest(t, 0x51),
	)
	if err != nil {
		t.Fatalf("NewMappingManifestRef() error = %v", err)
	}
	return manifest
}

func testProject(t *testing.T, raw string) projectidentity.ProjectID {
	t.Helper()
	project, err := projectidentity.ParseProjectID(raw)
	if err != nil {
		t.Fatalf("ParseProjectID() error = %v", err)
	}
	return project
}

func testEntity(t *testing.T, raw string) typedmemory.EntityID {
	t.Helper()
	entity, err := typedmemory.NewEntityID(raw)
	if err != nil {
		t.Fatalf("NewEntityID() error = %v", err)
	}
	return entity
}

func testContext(t *testing.T, raw string) typedmemory.BoundedContextRef {
	t.Helper()
	context, err := typedmemory.NewBoundedContextRef(raw)
	if err != nil {
		t.Fatalf("NewBoundedContextRef() error = %v", err)
	}
	return context
}

func testDigest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat(string([]byte{hexDigit(fill >> 4), hexDigit(fill & 0x0f)}), 32))
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	return digest
}

func hexDigit(value byte) byte {
	const digits = "0123456789abcdef"
	return digits[value]
}

func testObservableRef(t *testing.T, raw string) typedmemory.ObservableInputRef {
	t.Helper()
	ref, err := typedmemory.NewObservableInputRef(raw)
	if err != nil {
		t.Fatalf("NewObservableInputRef() error = %v", err)
	}
	return ref
}

func decodeCarrierDTO(
	t *testing.T,
	canonical []byte,
) projectRecordCarrierCanonicalV1 {
	t.Helper()
	var value projectRecordCarrierCanonicalV1
	if err := json.Unmarshal(canonical, &value); err != nil {
		t.Fatalf("json.Unmarshal(carrier) error = %v", err)
	}
	return value
}

func decodeBindingDTO(
	t *testing.T,
	canonical []byte,
) entityRecordCarrierBindingCanonicalV1 {
	t.Helper()
	var value entityRecordCarrierBindingCanonicalV1
	if err := json.Unmarshal(canonical, &value); err != nil {
		t.Fatalf("json.Unmarshal(binding) error = %v", err)
	}
	return value
}

func decodeSourceDTO(
	t *testing.T,
	canonical []byte,
) recordMembershipSourceCanonicalV1 {
	t.Helper()
	var value recordMembershipSourceCanonicalV1
	if err := json.Unmarshal(canonical, &value); err != nil {
		t.Fatalf("json.Unmarshal(source) error = %v", err)
	}
	return value
}

func mutateBinding(
	t *testing.T,
	binding EntityRecordCarrierBindingV1,
	mutate func(*entityRecordCarrierBindingCanonicalV1),
) EntityRecordCarrierBindingV1 {
	t.Helper()
	value := decodeBindingDTO(t, binding.CanonicalBytes())
	mutate(&value)
	decoded, err := DecodeEntityRecordCarrierBindingV1(marshalCanonicalTest(t, value))
	if err != nil {
		t.Fatalf("DecodeEntityRecordCarrierBindingV1(mutated) error = %v", err)
	}
	return decoded
}

func marshalCanonicalTest(t *testing.T, value any) []byte {
	t.Helper()
	canonical, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return canonical
}
