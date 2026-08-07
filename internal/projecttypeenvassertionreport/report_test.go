package projecttypeenvassertionreport

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCanonicalReaderRejectsUnrepresentableFieldLength(t *testing.T) {
	t.Parallel()
	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, math.MaxUint64)
	reader := &canonicalReader{raw: raw}

	_, err := reader.readBytes()
	if err == nil || err.Error() != "canonical field is truncated" {
		t.Fatalf("readBytes() error = %v, want canonical field is truncated", err)
	}
}

func TestGraphSnapshotCoordinateRejectsMismatchAndRoundTrips(t *testing.T) {
	t.Parallel()
	ref := reportGraphSnapshotRef(t, "1")
	coordinate, err := NewGraphSnapshotCoordinate(
		ref,
		typedmemory.NewGraphRevision(7),
		ref.Digest(),
	)
	if err != nil {
		t.Fatalf("NewGraphSnapshotCoordinate() error = %v", err)
	}
	if err := coordinate.Verify(); err != nil {
		t.Fatalf("GraphSnapshotCoordinate.Verify() error = %v", err)
	}
	decoded, err := DecodeCanonicalGraphSnapshotCoordinate(
		coordinate.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf(
			"DecodeCanonicalGraphSnapshotCoordinate() error = %v",
			err,
		)
	}
	if decoded.Ref() != ref ||
		decoded.Revision() != typedmemory.NewGraphRevision(7) ||
		decoded.BasisDigest() != ref.Digest() {
		t.Fatal("graph snapshot coordinate round-trip changed an exact field")
	}
	if _, err := NewGraphSnapshotCoordinate(
		ref,
		typedmemory.NewGraphRevision(7),
		reportDigest(t, "2"),
	); err == nil {
		t.Fatal("graph snapshot coordinate accepted a mismatched basis digest")
	}

	mutable := coordinate.CanonicalBytes()
	mutable[0] ^= 0xff
	if bytes.Equal(mutable, coordinate.CanonicalBytes()) {
		t.Fatal("graph snapshot coordinate exposed mutable canonical bytes")
	}
}

func TestReportDerivesOrderPostureAndStrictCanonicalRoundTrip(t *testing.T) {
	t.Parallel()
	invalidGround := reportInvalidGround(t)
	missingGround := reportMissingGround(t)
	invalid := reportOutcome(
		t,
		"assertion:z-invalid",
		"3",
		[]Ground{invalidGround},
	)
	under := reportOutcome(
		t,
		"assertion:a-under",
		"4",
		[]Ground{missingGround},
	)
	valid := reportOutcome(
		t,
		"assertion:m-valid",
		"5",
		nil,
	)
	graphRef := reportGraphSnapshotRef(t, "6")
	graph := mustReportValue(NewGraphSnapshotCoordinate(
		graphRef,
		typedmemory.NewGraphRevision(9),
		graphRef.Digest(),
	))
	report := mustReportValue(NewReport(
		reportTypeEnvRef(t, "7"),
		graph,
		reportRuntimeBasisRef(t, "8"),
		reportDigest(t, "9"),
		[]AssertionOutcome{invalid, valid, under},
	))

	if report.Posture() != typedmemory.RevalidationConflict {
		t.Fatalf(
			"Posture() = %s, want conflict",
			report.Posture().String(),
		)
	}
	outcomes := report.Outcomes()
	if len(outcomes) != 3 ||
		outcomes[0].AssertionID().String() != "assertion:a-under" ||
		outcomes[1].AssertionID().String() != "assertion:m-valid" ||
		outcomes[2].AssertionID().String() != "assertion:z-invalid" {
		t.Fatalf("canonical outcome order = %v", reportOutcomeIDs(outcomes))
	}
	affected := report.AffectedAssertions()
	if len(affected) != 2 ||
		affected[0].String() != "assertion:a-under" ||
		affected[1].String() != "assertion:z-invalid" {
		t.Fatalf("affected assertions = %v", reportAssertionIDs(affected))
	}
	if err := report.Verify(); err != nil {
		t.Fatalf("Report.Verify() error = %v", err)
	}
	decoded, err := DecodeCanonicalReport(report.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeCanonicalReport() error = %v", err)
	}
	if decoded.Digest() != report.Digest() ||
		!bytes.Equal(decoded.CanonicalBytes(), report.CanonicalBytes()) {
		t.Fatal("canonical report round-trip changed identity")
	}

	reordered := mustReportValue(NewReport(
		report.TargetTypeEnv(),
		report.GraphSnapshot(),
		report.RuntimeBasisRef(),
		report.RuntimeCoordinateDigest(),
		[]AssertionOutcome{under, invalid, valid},
	))
	if reordered.Digest() != report.Digest() {
		t.Fatal("outcome permutation changed report identity")
	}
	if _, err := DecodeCanonicalReport(
		append(report.CanonicalBytes(), 0x01),
	); err == nil {
		t.Fatal("DecodeCanonicalReport() accepted trailing bytes")
	}
}

func reportInvalidGround(t *testing.T) Ground {
	t.Helper()
	expected := mustReportValue(NewGroundDetail(
		"expected",
		[]string{"one"},
	))
	actual := mustReportValue(NewGroundDetail(
		"actual",
		[]string{"two"},
	))
	return mustReportValue(NewInvalidGround(
		CodeCardinalityMismatch,
		"assertions.test.slots.payload",
		"the persisted cardinality conflicts with the target signature",
		[]GroundDetail{expected, actual},
	))
}

func reportMissingGround(t *testing.T) Ground {
	t.Helper()
	required := mustReportValue(NewGroundDetail(
		"required",
		[]string{"target-signature"},
	))
	return mustReportValue(NewMissingBasisGround(
		CodeTargetSignatureUnavailable,
		"assertions.test.signature",
		"the exact target signature is unavailable",
		[]GroundDetail{required},
		"inspect-target-signature",
	))
}

func reportOutcome(
	t *testing.T,
	assertionRaw string,
	digestDigit string,
	grounds []Ground,
) AssertionOutcome {
	t.Helper()
	assertion := mustReportValue(typedmemory.NewAssertionID(assertionRaw))
	return mustReportValue(NewAssertionOutcome(
		assertion,
		reportDigest(t, digestDigit),
		grounds,
	))
}

func reportGraphSnapshotRef(
	t *testing.T,
	digit string,
) GraphSnapshotRef {
	t.Helper()
	return mustReportValue(ParseGraphSnapshotRef(
		"project-graph-snapshot-basis:sha256:" +
			strings.Repeat(digit, 64),
	))
}

func reportTypeEnvRef(
	t *testing.T,
	digit string,
) typedmemory.TypeEnvRef {
	t.Helper()
	return mustReportValue(typedmemory.ParseTypeEnvRef(
		"typeenv:sha256:" + strings.Repeat(digit, 64),
	))
}

func reportRuntimeBasisRef(
	t *testing.T,
	digit string,
) projecttypeenv.RuntimeEvaluationBasisRef {
	t.Helper()
	return mustReportValue(projecttypeenv.ParseRuntimeEvaluationBasisRef(
		"runtime-evaluation-basis:sha256:" +
			strings.Repeat(digit, 64),
	))
}

func reportDigest(
	t *testing.T,
	digit string,
) typedmemory.SHA256Digest {
	t.Helper()
	return mustReportValue(typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat(digit, 64),
	))
}

func reportOutcomeIDs(outcomes []AssertionOutcome) []string {
	result := make([]string, 0, len(outcomes))
	for _, outcome := range outcomes {
		result = append(result, outcome.AssertionID().String())
	}
	return result
}

func reportAssertionIDs(assertions []typedmemory.AssertionID) []string {
	result := make([]string, 0, len(assertions))
	for _, assertion := range assertions {
		result = append(result, assertion.String())
	}
	return result
}

func mustReportValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
