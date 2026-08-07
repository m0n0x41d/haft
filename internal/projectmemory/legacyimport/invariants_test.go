package legacyimport

import (
	"bytes"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
)

func TestCarrierCatalogUsesStructuralLocatorIdentity(t *testing.T) {
	first := mustCarrierSnapshot(
		t,
		"source:first",
		"carrier:a\x00b",
		"c",
		"application/octet-stream",
		[]byte("first"),
		mustLegacyIdentity(t, "legacy:first"),
	)
	second := mustCarrierSnapshot(
		t,
		"source:second",
		"carrier:a",
		"b\x00c",
		"application/octet-stream",
		[]byte("second"),
		mustLegacyIdentity(t, "legacy:second"),
	)

	catalog, err := NewCarrierCatalog([]CarrierSnapshot{first, second})
	if err != nil {
		t.Fatalf("NewCarrierCatalog() error = %v", err)
	}
	if catalog.Len() != 2 {
		t.Fatalf("catalog length = %d, want 2 structurally distinct locators", catalog.Len())
	}
}

func TestCarrierCatalogRejectsOneCoordinateNamingTwoLocators(t *testing.T) {
	first := mustCarrierSnapshot(
		t,
		"artifacts/id=note-1",
		"carrier:artifact:note-1",
		"legacy-row:1",
		"text/markdown",
		[]byte("first bytes"),
		mustLegacyIdentity(t, "legacy:note:note-1"),
	)
	second := mustCarrierSnapshot(
		t,
		"artifacts/id=note-1",
		"carrier:artifact:note-2",
		"legacy-row:2",
		"text/markdown",
		[]byte("second bytes"),
		mustLegacyIdentity(t, "legacy:note:note-2"),
	)

	_, err := NewCarrierCatalog([]CarrierSnapshot{first, second})
	if !errors.Is(err, ErrSourceCoordinateConflict) {
		t.Fatalf("NewCarrierCatalog() error = %v, want ErrSourceCoordinateConflict", err)
	}
}

func TestDryRunReportRejectsMoreThanOneClassificationForSubject(t *testing.T) {
	carrier := testCarrier(t, "classification-collision")
	subject := mustSubject(t, "subject:classification-collision")
	observation := testCarrierObservation(t, subject, carrier)
	carrierOnly, err := NewCarrierOnly(subject, []CarrierObservation{observation})
	if err != nil {
		t.Fatalf("NewCarrierOnly() error = %v", err)
	}
	reason, err := NewUnresolvedReason("classifier_disagreement")
	if err != nil {
		t.Fatalf("NewUnresolvedReason() error = %v", err)
	}
	unresolved, err := NewUnresolved(subject, reason, []SubjectObservation{observation})
	if err != nil {
		t.Fatalf("NewUnresolved() error = %v", err)
	}

	_, err = newDryRunReportForTest(
		t,
		[]CarrierSnapshot{carrier},
		[]SubjectObservation{observation},
		[]SubjectClassification{carrierOnly, unresolved},
	)
	if !errors.Is(err, ErrClassificationCollision) {
		t.Fatalf("NewDryRunReport() error = %v, want ErrClassificationCollision", err)
	}
}

func TestDryRunReportRejectsUnclassifiedObservation(t *testing.T) {
	carrier := testCarrier(t, "unclassified")
	subject := mustSubject(t, "subject:unclassified")
	observation := testCarrierObservation(t, subject, carrier)

	_, err := newDryRunReportForTest(
		t,
		[]CarrierSnapshot{carrier},
		[]SubjectObservation{observation},
		nil,
	)
	if !errors.Is(err, ErrObservationUnclassified) {
		t.Fatalf("NewDryRunReport() error = %v, want ErrObservationUnclassified", err)
	}
}

func TestDryRunReportRejectsClassificationObservationDrift(t *testing.T) {
	firstCarrier := testCarrier(t, "drift-source")
	secondCarrier := testCarrier(t, "drift-classification")
	subject := mustSubject(t, "subject:drift")
	sourceObservation := testCarrierObservation(t, subject, firstCarrier)
	classifiedObservation := testCarrierObservation(t, subject, secondCarrier)
	classification, err := NewCarrierOnly(
		subject,
		[]CarrierObservation{classifiedObservation},
	)
	if err != nil {
		t.Fatalf("NewCarrierOnly() error = %v", err)
	}

	_, err = newDryRunReportForTest(
		t,
		[]CarrierSnapshot{firstCarrier, secondCarrier},
		[]SubjectObservation{sourceObservation},
		[]SubjectClassification{classification},
	)
	if !errors.Is(err, ErrClassificationObservationDrift) {
		t.Fatalf("NewDryRunReport() error = %v, want ErrClassificationObservationDrift", err)
	}
}

func TestLegacySourceSnapshotRejectsObservationFromUnknownCarrier(t *testing.T) {
	known := testCarrier(t, "known")
	unknown := testCarrier(t, "unknown")
	subject := mustSubject(t, "subject:unknown-carrier")
	observation := testCarrierObservation(t, subject, unknown)
	catalog, err := NewCarrierCatalog([]CarrierSnapshot{known})
	if err != nil {
		t.Fatalf("NewCarrierCatalog() error = %v", err)
	}
	observations, err := NewObservationSet([]SubjectObservation{observation})
	if err != nil {
		t.Fatalf("NewObservationSet() error = %v", err)
	}

	_, err = NewLegacySourceSnapshot(catalog, observations)
	if !errors.Is(err, ErrUnknownCarrier) {
		t.Fatalf("NewLegacySourceSnapshot() error = %v, want ErrUnknownCarrier", err)
	}
}

func TestOneCarrierMayDescribeMultipleSeparateSubjects(t *testing.T) {
	carrier := testCarrier(t, "shared")
	firstSubject := mustSubject(t, "subject:shared:first")
	secondSubject := mustSubject(t, "subject:shared:second")
	firstObservation := testCarrierObservation(t, firstSubject, carrier)
	secondObservation := testCarrierObservation(t, secondSubject, carrier)
	firstClassification, err := NewCarrierOnly(
		firstSubject,
		[]CarrierObservation{firstObservation},
	)
	if err != nil {
		t.Fatalf("NewCarrierOnly(first) error = %v", err)
	}
	secondClassification, err := NewCarrierOnly(
		secondSubject,
		[]CarrierObservation{secondObservation},
	)
	if err != nil {
		t.Fatalf("NewCarrierOnly(second) error = %v", err)
	}

	report, err := newDryRunReportForTest(
		t,
		[]CarrierSnapshot{carrier},
		[]SubjectObservation{firstObservation, secondObservation},
		[]SubjectClassification{firstClassification, secondClassification},
	)
	if err != nil {
		t.Fatalf("NewDryRunReport() error = %v", err)
	}
	if report.Summary().Total() != 2 || report.CarrierCatalog().Len() != 1 {
		t.Fatalf("report summary/catalog = %d/%d, want 2 separate subjects over 1 carrier", report.Summary().Total(), report.CarrierCatalog().Len())
	}
	if report.Items()[0].Subject() == report.Items()[1].Subject() {
		t.Fatal("separate subjects collapsed because they share one carrier")
	}
}

func TestDryRunReportOwnsInputsAndDefensivelyCopiesOutputs(t *testing.T) {
	carrier := testCarrier(t, "defensive-copy")
	subject := mustSubject(t, "subject:defensive-copy")
	observation := testCarrierObservation(t, subject, carrier)
	classification, err := NewCarrierOnly(subject, []CarrierObservation{observation})
	if err != nil {
		t.Fatalf("NewCarrierOnly() error = %v", err)
	}
	carriers := []CarrierSnapshot{carrier}
	observations := []SubjectObservation{observation}
	classifications := []SubjectClassification{classification}
	report, err := newDryRunReportForTest(t, carriers, observations, classifications)
	if err != nil {
		t.Fatalf("NewDryRunReport() error = %v", err)
	}
	wantCanonical := report.CanonicalBytes()
	wantDigest := report.Digest().String()

	carriers[0] = CarrierSnapshot{}
	observations[0] = nil
	classifications[0] = nil
	returnedBytes := report.CanonicalBytes()
	returnedBytes[0] ^= 0xff
	returnedItems := report.Items()
	returnedItems[0] = nil
	returnedCarriers := report.CarrierCatalog().Snapshots()
	returnedCarriers[0] = CarrierSnapshot{}
	returnedExactBytes := report.CarrierCatalog().Snapshots()[0].ExactBytes()
	returnedExactBytes[0] ^= 0xff
	returnedObservations := report.Items()[0].Observations()
	returnedObservations[0] = nil

	if !bytes.Equal(report.CanonicalBytes(), wantCanonical) {
		t.Fatal("report canonical bytes changed through an input or returned slice")
	}
	if report.Digest().String() != wantDigest {
		t.Fatalf("report digest changed through an input or returned slice: %s != %s", report.Digest().String(), wantDigest)
	}
	if report.Items()[0].Subject() != subject {
		t.Fatal("report item changed through a returned slice")
	}
	if string(report.CarrierCatalog().Snapshots()[0].ExactBytes()) != "carrier:defensive-copy" {
		t.Fatal("carrier exact bytes changed through a returned slice")
	}
}

func newDryRunReportForTest(
	t *testing.T,
	carriers []CarrierSnapshot,
	observations []SubjectObservation,
	classifications []SubjectClassification,
) (DryRunReport, error) {
	t.Helper()
	catalog, err := NewCarrierCatalog(carriers)
	if err != nil {
		return DryRunReport{}, err
	}
	observationSet, err := NewObservationSet(observations)
	if err != nil {
		return DryRunReport{}, err
	}
	source, err := NewLegacySourceSnapshot(catalog, observationSet)
	if err != nil {
		return DryRunReport{}, err
	}
	projectID, err := projectidentity.ParseProjectID("qnt_e3149c17")
	if err != nil {
		return DryRunReport{}, err
	}
	classifier, err := NewClassifierVersion("legacy-import-classifier.v1")
	if err != nil {
		return DryRunReport{}, err
	}
	return NewDryRunReport(projectID, classifier, source, classifications)
}

func testCarrier(t *testing.T, suffix string) CarrierSnapshot {
	t.Helper()
	return mustCarrierSnapshot(
		t,
		"source:"+suffix,
		"carrier:"+suffix,
		"edition:1",
		"application/octet-stream",
		[]byte("carrier:"+suffix),
		mustLegacyIdentity(t, "legacy:"+suffix),
	)
}

func testCarrierObservation(
	t *testing.T,
	subject SemanticSubjectRef,
	carrier CarrierSnapshot,
) CarrierObservation {
	t.Helper()
	observation, err := NewCarrierObservation(subject, carrier)
	if err != nil {
		t.Fatalf("NewCarrierObservation() error = %v", err)
	}
	return observation
}
