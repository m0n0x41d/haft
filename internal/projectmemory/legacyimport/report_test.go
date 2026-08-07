package legacyimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCarrierCatalogPreservesArtifactAndHolonWithOneLegacyIdentity(t *testing.T) {
	legacyIdentity := mustLegacyIdentity(t, "legacy:decision:dec-1")
	artifactBytes := []byte("artifact decision carrier")
	artifact := mustCarrierSnapshot(
		t,
		"artifacts/id=dec-1",
		"carrier:artifact:dec-1",
		"legacy-row:1",
		"application/json",
		artifactBytes,
		legacyIdentity,
	)
	holon := mustCarrierSnapshot(
		t,
		"holons/id=dec-1",
		"carrier:holon:dec-1",
		"legacy-row:1",
		"text/markdown",
		[]byte("holon decision body differs"),
		legacyIdentity,
	)

	artifactBytes[0] = 'X'
	if string(artifact.ExactBytes()) != "artifact decision carrier" {
		t.Fatal("carrier snapshot did not own its exact source bytes")
	}
	if artifact.Digest().String() == holon.Digest().String() {
		t.Fatal("distinct legacy carrier bytes collapsed to one digest")
	}

	catalog, err := NewCarrierCatalog([]CarrierSnapshot{holon, artifact})
	if err != nil {
		t.Fatalf("NewCarrierCatalog() error = %v", err)
	}
	if catalog.Len() != 2 {
		t.Fatalf("catalog length = %d, want 2", catalog.Len())
	}
	for _, snapshot := range catalog.Snapshots() {
		identified, ok := snapshot.LegacyIdentity().(IdentifiedLegacyCarrier)
		if !ok {
			t.Fatalf("legacy identity variant = %T, want IdentifiedLegacyCarrier", snapshot.LegacyIdentity())
		}
		if identified.Ref() != legacyIdentity {
			t.Fatalf("legacy identity = %q, want %q", identified.Ref().String(), legacyIdentity.String())
		}
	}
}

func TestFreeAssociationRemainsLegacyUnbound(t *testing.T) {
	relationCarrier := mustCarrierSnapshot(
		t,
		"artifact_links/rowid=52",
		"carrier:artifact-link:52",
		"legacy-row:52",
		"application/x-sqlite-row",
		[]byte(`{"source":"dec-1","target":"spec-1","type":"governs"}`),
		mustLegacyIdentity(t, "legacy:association:52"),
	)
	subject := mustSubject(t, "subject:association:artifact-link:52")
	observation := mustAssociationObservation(
		t,
		subject,
		relationCarrier,
		"legacy:artifact:dec-1",
		"legacy:spec:spec-1",
		"governs",
	)
	classification, err := NewLegacyUnbound(subject, []AssociationObservation{observation})
	if err != nil {
		t.Fatalf("NewLegacyUnbound() error = %v", err)
	}
	report := mustDryRunReport(
		t,
		[]CarrierSnapshot{relationCarrier},
		[]SubjectObservation{observation},
		[]SubjectClassification{classification},
	)

	if report.Summary().LegacyUnbound() != 1 {
		t.Fatalf("legacy_unbound count = %d, want 1", report.Summary().LegacyUnbound())
	}
	if got := report.Items()[0].Kind(); got != ClassificationLegacyUnbound {
		t.Fatalf("classification kind = %q, want %q", got, ClassificationLegacyUnbound)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal(report) error = %v", err)
	}
	if !bytes.Contains(encoded, []byte(`"authority":"read_only_no_write_no_admission"`)) {
		t.Fatalf("report authority boundary missing: %s", encoded)
	}
	if !bytes.Contains(encoded, []byte(`"report_digest":"sha256:`)) {
		t.Fatalf("report digest missing: %s", encoded)
	}
}

func TestEvidenceCarrierStaysSeparateFromUnresolvedSemanticRelation(t *testing.T) {
	evidenceCarrier := mustCarrierSnapshot(
		t,
		"evidence/id=ev-1",
		"carrier:evidence:ev-1",
		"legacy-row:1",
		"application/json",
		[]byte(`{"content":"latency improved"}`),
		mustLegacyIdentity(t, "legacy:evidence:ev-1"),
	)
	relationCarrier := mustCarrierSnapshot(
		t,
		"evidence_relations/rowid=1",
		"carrier:evidence-relation:1",
		"legacy-row:1",
		"application/x-sqlite-row",
		[]byte(`{"evidence":"ev-1","decision":"dec-1"}`),
		mustLegacyIdentity(t, "legacy:evidence-relation:1"),
	)
	evidenceSubject := mustSubject(t, "subject:evidence:ev-1")
	relationSubject := mustSubject(t, "subject:relation:evidence:ev-1:decision:dec-1")
	evidenceObservation, err := NewCarrierObservation(evidenceSubject, evidenceCarrier)
	if err != nil {
		t.Fatalf("NewCarrierObservation() error = %v", err)
	}
	relationObservation := mustAssociationObservation(
		t,
		relationSubject,
		relationCarrier,
		"legacy:evidence:ev-1",
		"legacy:decision:dec-1",
		"supports",
	)
	carrierOnly, err := NewCarrierOnly(evidenceSubject, []CarrierObservation{evidenceObservation})
	if err != nil {
		t.Fatalf("NewCarrierOnly() error = %v", err)
	}
	reason, err := NewUnresolvedReason("missing_exact_evidence_provenance")
	if err != nil {
		t.Fatalf("NewUnresolvedReason() error = %v", err)
	}
	unresolved, err := NewUnresolved(
		relationSubject,
		reason,
		[]SubjectObservation{relationObservation},
	)
	if err != nil {
		t.Fatalf("NewUnresolved() error = %v", err)
	}
	report := mustDryRunReport(
		t,
		[]CarrierSnapshot{relationCarrier, evidenceCarrier},
		[]SubjectObservation{relationObservation, evidenceObservation},
		[]SubjectClassification{unresolved, carrierOnly},
	)

	if report.Summary().CarrierOnly() != 1 || report.Summary().Unresolved() != 1 {
		t.Fatalf("summary = %#v, want one carrier_only and one unresolved", report.Summary())
	}
	if report.Summary().Total() != 2 {
		t.Fatalf("summary total = %d, want 2", report.Summary().Total())
	}
}

func TestCarrierCatalogRejectsSameLocatorWithConflictingBytes(t *testing.T) {
	first := mustCarrierSnapshot(
		t,
		"artifacts/id=note-1:first-read",
		"carrier:artifact:note-1",
		"legacy-row:1",
		"text/markdown",
		[]byte("first bytes"),
		mustLegacyIdentity(t, "legacy:note:note-1"),
	)
	second := mustCarrierSnapshot(
		t,
		"artifacts/id=note-1:second-read",
		"carrier:artifact:note-1",
		"legacy-row:1",
		"text/markdown",
		[]byte("changed bytes"),
		mustLegacyIdentity(t, "legacy:note:note-1"),
	)

	_, err := NewCarrierCatalog([]CarrierSnapshot{first, second})
	if !errors.Is(err, ErrCarrierCollision) {
		t.Fatalf("NewCarrierCatalog() error = %v, want ErrCarrierCollision", err)
	}
}

func TestDryRunReportIsPermutationStable(t *testing.T) {
	noteCarrier := mustCarrierSnapshot(
		t,
		"artifacts/id=note-1",
		"carrier:artifact:note-1",
		"legacy-row:1",
		"text/markdown",
		[]byte("operator note"),
		mustLegacyIdentity(t, "legacy:note:note-1"),
	)
	relationCarrier := mustCarrierSnapshot(
		t,
		"artifact_links/rowid=7",
		"carrier:artifact-link:7",
		"legacy-row:7",
		"application/x-sqlite-row",
		[]byte(`{"source":"note-1","target":"problem-1","type":"relates_to"}`),
		mustLegacyIdentity(t, "legacy:association:7"),
	)
	noteSubject := mustSubject(t, "subject:note:note-1")
	relationSubject := mustSubject(t, "subject:association:artifact-link:7")
	noteObservation, err := NewCarrierObservation(noteSubject, noteCarrier)
	if err != nil {
		t.Fatalf("NewCarrierObservation() error = %v", err)
	}
	relationObservation := mustAssociationObservation(
		t,
		relationSubject,
		relationCarrier,
		"legacy:note:note-1",
		"legacy:problem:problem-1",
		"relates_to",
	)
	noteClassification, err := NewCarrierOnly(noteSubject, []CarrierObservation{noteObservation})
	if err != nil {
		t.Fatalf("NewCarrierOnly() error = %v", err)
	}
	relationClassification, err := NewLegacyUnbound(
		relationSubject,
		[]AssociationObservation{relationObservation},
	)
	if err != nil {
		t.Fatalf("NewLegacyUnbound() error = %v", err)
	}

	first := mustDryRunReport(
		t,
		[]CarrierSnapshot{noteCarrier, relationCarrier},
		[]SubjectObservation{noteObservation, relationObservation},
		[]SubjectClassification{noteClassification, relationClassification},
	)
	second := mustDryRunReport(
		t,
		[]CarrierSnapshot{relationCarrier, noteCarrier},
		[]SubjectObservation{relationObservation, noteObservation},
		[]SubjectClassification{relationClassification, noteClassification},
	)

	if !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatalf("canonical dry-run reports differ by input permutation:\n%s\n%s", first.CanonicalBytes(), second.CanonicalBytes())
	}
	if first.Digest().String() != second.Digest().String() {
		t.Fatalf("report digests differ: %s != %s", first.Digest().String(), second.Digest().String())
	}
}

func mustDryRunReport(
	t *testing.T,
	carriers []CarrierSnapshot,
	observations []SubjectObservation,
	classifications []SubjectClassification,
) DryRunReport {
	t.Helper()
	catalog, err := NewCarrierCatalog(carriers)
	if err != nil {
		t.Fatalf("NewCarrierCatalog() error = %v", err)
	}
	observationSet, err := NewObservationSet(observations)
	if err != nil {
		t.Fatalf("NewObservationSet() error = %v", err)
	}
	source, err := NewLegacySourceSnapshot(catalog, observationSet)
	if err != nil {
		t.Fatalf("NewLegacySourceSnapshot() error = %v", err)
	}
	projectID, err := projectidentity.ParseProjectID("qnt_e3149c17")
	if err != nil {
		t.Fatalf("ParseProjectID() error = %v", err)
	}
	classifier, err := NewClassifierVersion("legacy-import-classifier.v1")
	if err != nil {
		t.Fatalf("NewClassifierVersion() error = %v", err)
	}
	report, err := NewDryRunReport(projectID, classifier, source, classifications)
	if err != nil {
		t.Fatalf("NewDryRunReport() error = %v", err)
	}
	return report
}

func mustCarrierSnapshot(
	t *testing.T,
	coordinateRaw string,
	refRaw string,
	editionRaw string,
	formatRaw string,
	exactBytes []byte,
	legacyIdentity LegacyIdentityRef,
) CarrierSnapshot {
	t.Helper()
	coordinate, err := NewSourceCoordinate(coordinateRaw)
	if err != nil {
		t.Fatalf("NewSourceCoordinate(%q) error = %v", coordinateRaw, err)
	}
	ref, err := typedmemory.NewCarrierRef(refRaw)
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierRef(%q) error = %v", refRaw, err)
	}
	edition, err := typedmemory.NewCarrierEdition(editionRaw)
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierEdition(%q) error = %v", editionRaw, err)
	}
	format, err := NewCarrierFormat(formatRaw)
	if err != nil {
		t.Fatalf("NewCarrierFormat(%q) error = %v", formatRaw, err)
	}
	identity, err := NewIdentifiedLegacyCarrier(legacyIdentity)
	if err != nil {
		t.Fatalf("NewIdentifiedLegacyCarrier() error = %v", err)
	}
	snapshot, err := NewCarrierSnapshot(
		coordinate,
		ref,
		edition,
		format,
		exactBytes,
		identity,
	)
	if err != nil {
		t.Fatalf("NewCarrierSnapshot() error = %v", err)
	}
	return snapshot
}

func mustAssociationObservation(
	t *testing.T,
	subject SemanticSubjectRef,
	carrier CarrierSnapshot,
	sourceRaw string,
	targetRaw string,
	labelRaw string,
) AssociationObservation {
	t.Helper()
	source := mustLegacyIdentity(t, sourceRaw)
	target := mustLegacyIdentity(t, targetRaw)
	label, err := NewAssociationLabel(labelRaw)
	if err != nil {
		t.Fatalf("NewAssociationLabel(%q) error = %v", labelRaw, err)
	}
	observation, err := NewAssociationObservation(subject, carrier, source, target, label)
	if err != nil {
		t.Fatalf("NewAssociationObservation() error = %v", err)
	}
	return observation
}

func mustLegacyIdentity(t *testing.T, raw string) LegacyIdentityRef {
	t.Helper()
	ref, err := NewLegacyIdentityRef(raw)
	if err != nil {
		t.Fatalf("NewLegacyIdentityRef(%q) error = %v", raw, err)
	}
	return ref
}

func mustSubject(t *testing.T, raw string) SemanticSubjectRef {
	t.Helper()
	ref, err := NewSemanticSubjectRef(raw)
	if err != nil {
		t.Fatalf("NewSemanticSubjectRef(%q) error = %v", raw, err)
	}
	return ref
}
