package projectgraphobservation_test

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestRevisionZeroObservationIsExactAndImmutable(t *testing.T) {
	project := mustProject(t, "qnt_1234abcd")
	basis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: typedmemory.NewGraphRevision(0),
			Closure:       projecttypeenvselection.EmptyProjectGraphClosure{},
		},
	)
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis: %v", err)
	}
	active, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		project,
		typedmemory.NewGraphRevision(0),
		nil,
	)
	if err != nil {
		t.Fatalf("NewCurrentActiveAssertionSet: %v", err)
	}
	observation, err := projectgraphobservation.NewCurrentProjectGraphObservation(
		basis,
		mustTypeEnv(t),
		active,
	)
	if err != nil {
		t.Fatalf("NewCurrentProjectGraphObservation: %v", err)
	}
	if err := observation.Verify(); err != nil {
		t.Fatalf("observation.Verify: %v", err)
	}
	relations := observation.ActiveAssertions().Relations()
	relations = append(relations, projectgraphobservation.CurrentActiveAssertion{})
	if len(observation.ActiveAssertions().Relations()) != 0 {
		t.Fatal("observation exposed mutable relation storage")
	}
}

func TestObservationRejectsCrossProjectAndRevisionZeroRelations(t *testing.T) {
	project := mustProject(t, "qnt_1234abcd")
	other := mustProject(t, "qnt_abcd1234")
	basis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: typedmemory.NewGraphRevision(0),
			Closure:       projecttypeenvselection.EmptyProjectGraphClosure{},
		},
	)
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis: %v", err)
	}
	active, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		other,
		typedmemory.NewGraphRevision(0),
		nil,
	)
	if err != nil {
		t.Fatalf("NewCurrentActiveAssertionSet: %v", err)
	}
	if _, err := projectgraphobservation.NewCurrentProjectGraphObservation(
		basis,
		mustTypeEnv(t),
		active,
	); err == nil {
		t.Fatal("cross-project graph observation was accepted")
	}
	if _, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		project,
		typedmemory.NewGraphRevision(0),
		[]projectgraphobservation.CurrentActiveAssertion{{}},
	); err == nil {
		t.Fatal("revision-zero graph accepted an assertion")
	}
}

func TestCurrentActiveAssertionSetOrdersAndRejectsDuplicateAssertions(
	t *testing.T,
) {
	project := mustProject(t, "qnt_1234abcd")
	higher := mustCurrentActiveAssertion(
		t,
		"assertion:z",
		"b",
		typedmemory.NewGraphRevision(2),
		1,
	)
	lower := mustCurrentActiveAssertion(
		t,
		"assertion:a",
		"a",
		typedmemory.NewGraphRevision(1),
		0,
	)
	input := []projectgraphobservation.CurrentActiveAssertion{higher, lower}
	set, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		project,
		typedmemory.NewGraphRevision(2),
		input,
	)
	if err != nil {
		t.Fatalf("NewCurrentActiveAssertionSet: %v", err)
	}
	if input[0].AssertionID().String() != "assertion:z" {
		t.Fatal("active assertion constructor reordered caller-owned input")
	}
	ordered := set.Relations()
	if ordered[0].AssertionID().String() != "assertion:a" ||
		ordered[1].AssertionID().String() != "assertion:z" {
		t.Fatalf(
			"canonical assertion order = [%s, %s]",
			ordered[0].AssertionID().String(),
			ordered[1].AssertionID().String(),
		)
	}
	_, err = projectgraphobservation.NewCurrentActiveAssertionSet(
		project,
		typedmemory.NewGraphRevision(2),
		[]projectgraphobservation.CurrentActiveAssertion{lower, lower},
	)
	if err == nil {
		t.Fatal("active assertion set accepted a duplicate assertion ID")
	}
}

func TestCurrentActiveRelationalAssertionPreservesExplicitModalityWithoutLegacyInference(
	t *testing.T,
) {
	assertion := mustCanonicalRelationalAssertion(
		t,
		"assertion:v3",
		typedmemory.AssertionModalityDeniesObtaining,
	)
	canonical, err := assertion.CanonicalBytes()
	if err != nil {
		t.Fatalf("RelationalAssertion.CanonicalBytes: %v", err)
	}
	digest, err := assertion.Digest()
	if err != nil {
		t.Fatalf("RelationalAssertion.Digest: %v", err)
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(
		"typed-memory-event:" + strings.Repeat("c", 64),
	)
	if err != nil {
		t.Fatalf("ParseGraphEventRef: %v", err)
	}
	active, err := projectgraphobservation.NewCurrentActiveRelationalAssertion(
		projectgraphobservation.CurrentActiveRelationalAssertionInput{
			Assertion:      assertion,
			CanonicalBytes: canonical,
			Digest:         digest,
			OriginEvent:    event,
			OriginRevision: typedmemory.NewGraphRevision(3),
			ChangeOrdinal:  2,
		},
	)
	if err != nil {
		t.Fatalf("NewCurrentActiveRelationalAssertion: %v", err)
	}
	if err := active.Verify(); err != nil {
		t.Fatalf("CurrentActiveAssertion.Verify: %v", err)
	}
	if _, legacy := active.LegacyRelation(); legacy {
		t.Fatal("v3 assertion exposed a fabricated legacy RelationInstance")
	}
	stored, v3 := active.RelationalAssertion()
	if !v3 || stored.Modality().Kind() != typedmemory.AssertionModalityDeniesObtaining {
		t.Fatal("v3 assertion carrier lost its exact explicit modality")
	}
	modality, explicit := active.Posture().ExplicitModality()
	if !explicit || modality != typedmemory.AssertionModalityDeniesObtaining {
		t.Fatalf(
			"current posture modality = (%q, %t); want explicit denies_obtaining",
			modality.String(),
			explicit,
		)
	}
	carrier, ok := active.Carrier().(projectgraphobservation.CurrentRelationalAssertion)
	if !ok || carrier.Kind() != projectgraphobservation.CurrentRelationalAssertionV3Carrier {
		t.Fatalf("current carrier = %T; want exact v3 variant", active.Carrier())
	}
	if carrier.RelationDeclarationPosture() !=
		typedmemory.RelationDeclarationTypedFragment {
		t.Fatalf(
			"current relation declaration posture = %q",
			carrier.RelationDeclarationPosture(),
		)
	}
}

func mustCurrentActiveAssertion(
	t *testing.T,
	assertionText string,
	eventDigit string,
	revision typedmemory.GraphRevision,
	changeOrdinal uint64,
) projectgraphobservation.CurrentActiveAssertion {
	t.Helper()
	relation := mustCanonicalRelation(t, assertionText)
	canonical, err := relation.CanonicalBytes()
	if err != nil {
		t.Fatalf("RelationInstance.CanonicalBytes: %v", err)
	}
	digest, err := relation.Digest()
	if err != nil {
		t.Fatalf("RelationInstance.Digest: %v", err)
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(
		"typed-memory-event:" + strings.Repeat(eventDigit, 64),
	)
	if err != nil {
		t.Fatalf("ParseGraphEventRef: %v", err)
	}
	assertion, err := projectgraphobservation.NewCurrentActiveAssertion(
		projectgraphobservation.CurrentActiveAssertionInput{
			Relation:       relation,
			CanonicalBytes: canonical,
			Digest:         digest,
			OriginEvent:    event,
			OriginRevision: revision,
			ChangeOrdinal:  changeOrdinal,
		},
	)
	if err != nil {
		t.Fatalf("NewCurrentActiveAssertion: %v", err)
	}
	return assertion
}

func mustCanonicalRelation(
	t *testing.T,
	assertionText string,
) typedmemory.RelationInstance {
	t.Helper()
	typeEnv := mustTypeEnv(t)
	signatureID, err := typedmemory.NewSignatureID("test.Relation")
	if err != nil {
		t.Fatalf("NewSignatureID: %v", err)
	}
	signature, err := typedmemory.NewRelationSignatureRef(typeEnv, signatureID)
	if err != nil {
		t.Fatalf("NewRelationSignatureRef: %v", err)
	}
	context, err := typedmemory.NewBoundedContextRef("ctx:test")
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewGammaPoint: %v", err)
	}
	slice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   context,
		GammaTime: gamma,
	})
	if err != nil {
		t.Fatalf("NewContextSlice: %v", err)
	}
	refKindID, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatalf("NewRefKindID: %v", err)
	}
	refKind, err := typedmemory.NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatalf("NewRefKindRef: %v", err)
	}
	referenceID, err := typedmemory.NewReferenceID("entity:target")
	if err != nil {
		t.Fatalf("NewReferenceID: %v", err)
	}
	reference, err := typedmemory.NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef: %v", err)
	}
	filler := canonicalTestEnvelope(
		"validated-by-reference.v2",
		[]byte(reference.RefKind().String()),
		[]byte(reference.ReferenceKey()),
		[]byte("entity:target"),
	)
	binding := canonicalTestEnvelope(
		"validated-slot-binding.v1",
		[]byte("entity"),
		filler,
	)
	raw := canonicalTestEnvelope(
		"validated-relation-instance.v2",
		[]byte(assertionText),
		[]byte(signature.String()),
		[]byte(slice.Ref().String()),
		slice.CanonicalBytes(),
		binding,
		[]byte("memory:test:project-graph-observation"),
	)
	relation, err := typedmemory.DecodeCanonicalRelationInstance(raw)
	if err != nil {
		t.Fatalf("DecodeCanonicalRelationInstance: %v", err)
	}
	return relation
}

func mustCanonicalRelationalAssertion(
	t *testing.T,
	assertionText string,
	modality typedmemory.AssertionModalityKind,
) typedmemory.RelationalAssertion {
	t.Helper()
	legacy := mustCanonicalRelation(t, assertionText)
	fields := [][]byte{
		[]byte(assertionText),
		[]byte(legacy.Signature().String()),
		[]byte(legacy.Slice().Ref().String()),
		legacy.Slice().CanonicalBytes(),
		[]byte(modality.String()),
	}
	for _, binding := range legacy.Bindings() {
		fields = append(fields, binding.CanonicalBytes())
	}
	fields = append(fields, []byte(legacy.Provenance().String()))
	raw := canonicalTestEnvelope(
		"validated-relational-assertion.v3",
		fields...,
	)
	assertion, err := typedmemory.DecodeCanonicalRelationalAssertion(raw)
	if err != nil {
		t.Fatalf("DecodeCanonicalRelationalAssertion: %v", err)
	}
	return assertion
}

func canonicalTestEnvelope(domain string, fields ...[]byte) []byte {
	buffer := bytes.Buffer{}
	appendCanonicalTestField(
		&buffer,
		[]byte("haft.typedmemory.canonical-envelope.v1"),
	)
	appendCanonicalTestField(&buffer, []byte(domain))
	for _, field := range fields {
		appendCanonicalTestField(&buffer, field)
	}
	return buffer.Bytes()
}

func appendCanonicalTestField(buffer *bytes.Buffer, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	buffer.Write(length[:])
	buffer.Write(value)
}

func mustProject(t *testing.T, raw string) projectidentity.ProjectID {
	t.Helper()
	project, err := projectidentity.ParseProjectID(raw)
	if err != nil {
		t.Fatalf("ParseProjectID: %v", err)
	}
	return project
}

func mustTypeEnv(t *testing.T) typedmemory.TypeEnvRef {
	t.Helper()
	ref, err := typedmemory.ParseTypeEnvRef(
		"typeenv:sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil {
		t.Fatalf("ParseTypeEnvRef: %v", err)
	}
	return ref
}
