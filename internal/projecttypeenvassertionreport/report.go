package projecttypeenvassertionreport

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// AssertionOutcomeKind is the closed existing-assertion result algebra.
type AssertionOutcomeKind uint8

const (
	AssertionValid AssertionOutcomeKind = iota + 1
	AssertionInvalid
	AssertionUnderdetermined
)

func (kind AssertionOutcomeKind) String() string {
	switch kind {
	case AssertionValid:
		return "valid"
	case AssertionInvalid:
		return "invalid"
	case AssertionUnderdetermined:
		return "underdetermined"
	default:
		return ""
	}
}

// AssertionOutcome is sealed. NewAssertionOutcome derives the exact variant,
// canonical ground set, and digest; callers cannot implement another variant
// or supply those derived fields.
type AssertionOutcome interface {
	Kind() AssertionOutcomeKind
	AssertionID() typedmemory.AssertionID
	RelationDigest() typedmemory.SHA256Digest
	Grounds() []Ground
	CanonicalBytes() []byte
	Digest() typedmemory.SHA256Digest
	assertionOutcomeVariant()
}

type assertionOutcome struct {
	kind           AssertionOutcomeKind
	assertion      typedmemory.AssertionID
	relationDigest typedmemory.SHA256Digest
	grounds        []Ground
	canonicalBytes []byte
	digest         typedmemory.SHA256Digest
}

type validAssertionOutcome struct{ assertionOutcome }
type invalidAssertionOutcome struct{ assertionOutcome }
type underdeterminedAssertionOutcome struct{ assertionOutcome }

func (outcome validAssertionOutcome) Kind() AssertionOutcomeKind {
	return outcome.kind
}

func (outcome validAssertionOutcome) AssertionID() typedmemory.AssertionID {
	return outcome.assertion
}

func (outcome validAssertionOutcome) RelationDigest() typedmemory.SHA256Digest {
	return outcome.relationDigest
}

func (outcome validAssertionOutcome) Grounds() []Ground {
	return nil
}

func (outcome validAssertionOutcome) CanonicalBytes() []byte {
	return append([]byte(nil), outcome.canonicalBytes...)
}

func (outcome validAssertionOutcome) Digest() typedmemory.SHA256Digest {
	return outcome.digest
}

func (validAssertionOutcome) assertionOutcomeVariant() {}

func (outcome invalidAssertionOutcome) Kind() AssertionOutcomeKind {
	return outcome.kind
}

func (outcome invalidAssertionOutcome) AssertionID() typedmemory.AssertionID {
	return outcome.assertion
}

func (outcome invalidAssertionOutcome) RelationDigest() typedmemory.SHA256Digest {
	return outcome.relationDigest
}

func (outcome invalidAssertionOutcome) Grounds() []Ground {
	return copyGrounds(outcome.grounds)
}

func (outcome invalidAssertionOutcome) CanonicalBytes() []byte {
	return append([]byte(nil), outcome.canonicalBytes...)
}

func (outcome invalidAssertionOutcome) Digest() typedmemory.SHA256Digest {
	return outcome.digest
}

func (invalidAssertionOutcome) assertionOutcomeVariant() {}

func (outcome underdeterminedAssertionOutcome) Kind() AssertionOutcomeKind {
	return outcome.kind
}

func (outcome underdeterminedAssertionOutcome) AssertionID() typedmemory.AssertionID {
	return outcome.assertion
}

func (outcome underdeterminedAssertionOutcome) RelationDigest() typedmemory.SHA256Digest {
	return outcome.relationDigest
}

func (outcome underdeterminedAssertionOutcome) Grounds() []Ground {
	return copyGrounds(outcome.grounds)
}

func (outcome underdeterminedAssertionOutcome) CanonicalBytes() []byte {
	return append([]byte(nil), outcome.canonicalBytes...)
}

func (outcome underdeterminedAssertionOutcome) Digest() typedmemory.SHA256Digest {
	return outcome.digest
}

func (underdeterminedAssertionOutcome) assertionOutcomeVariant() {}

func NewAssertionOutcome(
	assertion typedmemory.AssertionID,
	relationDigest typedmemory.SHA256Digest,
	grounds []Ground,
) (AssertionOutcome, error) {
	canonicalAssertion, err := typedmemory.NewAssertionID(assertion.String())
	if err != nil || canonicalAssertion != assertion {
		return nil, fmt.Errorf("assertion outcome requires an exact assertion ID")
	}
	canonicalRelationDigest, err := typedmemory.NewSHA256Digest(
		relationDigest.String(),
	)
	if err != nil || canonicalRelationDigest != relationDigest {
		return nil, fmt.Errorf("assertion outcome requires an exact relation digest")
	}
	normalized, err := normalizeGrounds(grounds)
	if err != nil {
		return nil, err
	}
	kind := deriveAssertionOutcomeKind(normalized)
	if kind.String() == "" {
		return nil, fmt.Errorf("assertion outcome could not be derived")
	}
	canonical := canonicalAssertionOutcome(
		kind,
		canonicalAssertion,
		canonicalRelationDigest,
		normalized,
	)
	digest, err := digestBytes(canonical)
	if err != nil {
		return nil, err
	}
	state := assertionOutcome{
		kind:           kind,
		assertion:      canonicalAssertion,
		relationDigest: canonicalRelationDigest,
		grounds:        normalized,
		canonicalBytes: canonical,
		digest:         digest,
	}
	switch kind {
	case AssertionValid:
		return validAssertionOutcome{assertionOutcome: state}, nil
	case AssertionInvalid:
		return invalidAssertionOutcome{assertionOutcome: state}, nil
	case AssertionUnderdetermined:
		return underdeterminedAssertionOutcome{assertionOutcome: state}, nil
	default:
		return nil, fmt.Errorf("assertion outcome kind is unsupported")
	}
}

func deriveAssertionOutcomeKind(grounds []Ground) AssertionOutcomeKind {
	for _, ground := range grounds {
		if ground.Posture() == GroundInvalid {
			return AssertionInvalid
		}
	}
	if len(grounds) > 0 {
		return AssertionUnderdetermined
	}
	return AssertionValid
}

func canonicalAssertionOutcome(
	kind AssertionOutcomeKind,
	assertion typedmemory.AssertionID,
	relationDigest typedmemory.SHA256Digest,
	grounds []Ground,
) []byte {
	writer := newCanonicalWriter(
		"haft.project-typeenv.assertion-revalidation-outcome.v1",
	)
	writer.addString(kind.String())
	writer.addString(assertion.String())
	writer.addString(relationDigest.String())
	writer.addUint64(uint64(len(grounds)))
	for _, ground := range grounds {
		writer.addBytes(canonicalGround(ground))
	}
	return writer.bytes()
}

func copyGrounds(grounds []Ground) []Ground {
	result := make([]Ground, 0, len(grounds))
	for _, ground := range grounds {
		details := ground.Details()
		result = append(result, Ground{
			posture: ground.posture,
			code:    ground.code,
			path:    ground.path,
			message: ground.message,
			details: details,
			repair:  ground.repair,
		})
	}
	return result
}

// Report is the immutable, internally derived complete revalidation result.
type Report struct {
	targetTypeEnv     typedmemory.TypeEnvRef
	graphSnapshot     GraphSnapshotCoordinate
	runtimeBasis      projecttypeenv.RuntimeEvaluationBasisRef
	runtimeCoordinate typedmemory.SHA256Digest
	outcomes          []AssertionOutcome
	posture           typedmemory.RevalidationPosture
	affected          []typedmemory.AssertionID
	canonicalBytes    []byte
	digest            typedmemory.SHA256Digest
}

func (report Report) TargetTypeEnv() typedmemory.TypeEnvRef {
	return report.targetTypeEnv
}

func (report Report) GraphSnapshotRef() GraphSnapshotRef {
	return report.graphSnapshot.Ref()
}

func (report Report) GraphRevision() typedmemory.GraphRevision {
	return report.graphSnapshot.Revision()
}

func (report Report) GraphSnapshot() GraphSnapshotCoordinate {
	return report.graphSnapshot
}

func (report Report) RuntimeBasisRef() projecttypeenv.RuntimeEvaluationBasisRef {
	return report.runtimeBasis
}

func (report Report) RuntimeCoordinateDigest() typedmemory.SHA256Digest {
	return report.runtimeCoordinate
}

func (report Report) Outcomes() []AssertionOutcome {
	return append([]AssertionOutcome(nil), report.outcomes...)
}

func (report Report) Posture() typedmemory.RevalidationPosture {
	return report.posture
}

func (report Report) AffectedAssertions() []typedmemory.AssertionID {
	return append([]typedmemory.AssertionID(nil), report.affected...)
}

func (report Report) CanonicalBytes() []byte {
	return append([]byte(nil), report.canonicalBytes...)
}

func (report Report) Digest() typedmemory.SHA256Digest {
	return report.digest
}

func (report Report) Verify() error {
	rebuilt, err := NewReport(
		report.targetTypeEnv,
		report.graphSnapshot,
		report.runtimeBasis,
		report.runtimeCoordinate,
		report.outcomes,
	)
	if err != nil {
		return err
	}
	if !bytes.Equal(rebuilt.canonicalBytes, report.canonicalBytes) ||
		rebuilt.digest != report.digest ||
		rebuilt.posture != report.posture ||
		!sameAssertionIDs(rebuilt.affected, report.affected) {
		return fmt.Errorf("assertion revalidation report state is inconsistent")
	}
	return nil
}

func NewReport(
	targetTypeEnv typedmemory.TypeEnvRef,
	graphSnapshot GraphSnapshotCoordinate,
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisRef,
	runtimeCoordinate typedmemory.SHA256Digest,
	outcomes []AssertionOutcome,
) (Report, error) {
	canonicalTypeEnv, err := typedmemory.ParseTypeEnvRef(targetTypeEnv.String())
	if err != nil || canonicalTypeEnv != targetTypeEnv {
		return Report{}, fmt.Errorf("revalidation report target TypeEnv is required")
	}
	if err := graphSnapshot.Verify(); err != nil {
		return Report{}, fmt.Errorf("revalidation report graph snapshot: %w", err)
	}
	canonicalRuntime, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(
		runtimeBasis.String(),
	)
	if err != nil || canonicalRuntime != runtimeBasis {
		return Report{}, fmt.Errorf("revalidation report runtime basis is required")
	}
	canonicalCoordinate, err := typedmemory.NewSHA256Digest(
		runtimeCoordinate.String(),
	)
	if err != nil || canonicalCoordinate != runtimeCoordinate {
		return Report{}, fmt.Errorf(
			"revalidation report runtime coordinate digest is required",
		)
	}
	normalized, err := normalizeOutcomes(outcomes)
	if err != nil {
		return Report{}, err
	}
	posture, affected := deriveReportProjection(normalized)
	canonical := canonicalReport(
		canonicalTypeEnv,
		graphSnapshot,
		canonicalRuntime,
		canonicalCoordinate,
		normalized,
	)
	digest, err := digestBytes(canonical)
	if err != nil {
		return Report{}, err
	}
	return Report{
		targetTypeEnv:     canonicalTypeEnv,
		graphSnapshot:     graphSnapshot,
		runtimeBasis:      canonicalRuntime,
		runtimeCoordinate: canonicalCoordinate,
		outcomes:          normalized,
		posture:           posture,
		affected:          affected,
		canonicalBytes:    canonical,
		digest:            digest,
	}, nil
}

func normalizeOutcomes(
	outcomes []AssertionOutcome,
) ([]AssertionOutcome, error) {
	if len(outcomes) > maximumCanonicalElements {
		return nil, fmt.Errorf("assertion outcome count exceeds the supported bound")
	}
	owned := append([]AssertionOutcome(nil), outcomes...)
	for index, outcome := range owned {
		if outcome == nil {
			return nil, fmt.Errorf("assertion outcome %d is nil", index)
		}
		rebuilt, err := NewAssertionOutcome(
			outcome.AssertionID(),
			outcome.RelationDigest(),
			outcome.Grounds(),
		)
		if err != nil {
			return nil, fmt.Errorf("assertion outcome %d: %w", index, err)
		}
		if rebuilt.Kind() != outcome.Kind() ||
			!bytes.Equal(rebuilt.CanonicalBytes(), outcome.CanonicalBytes()) ||
			rebuilt.Digest() != outcome.Digest() {
			return nil, fmt.Errorf(
				"assertion outcome %d stored state is inconsistent",
				index,
			)
		}
		owned[index] = rebuilt
	}
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].AssertionID().String() <
			owned[right].AssertionID().String()
	})
	for index := 1; index < len(owned); index++ {
		if owned[index-1].AssertionID() == owned[index].AssertionID() {
			return nil, fmt.Errorf(
				"revalidation report repeats assertion %q",
				owned[index].AssertionID().String(),
			)
		}
	}
	return owned, nil
}

func deriveReportProjection(
	outcomes []AssertionOutcome,
) (typedmemory.RevalidationPosture, []typedmemory.AssertionID) {
	posture := typedmemory.RevalidationClean
	affected := make([]typedmemory.AssertionID, 0)
	for _, outcome := range outcomes {
		switch outcome.Kind() {
		case AssertionInvalid:
			posture = typedmemory.RevalidationConflict
			affected = append(affected, outcome.AssertionID())
		case AssertionUnderdetermined:
			if posture != typedmemory.RevalidationConflict {
				posture = typedmemory.RevalidationUnderdetermined
			}
			affected = append(affected, outcome.AssertionID())
		}
	}
	return posture, affected
}

func canonicalReport(
	targetTypeEnv typedmemory.TypeEnvRef,
	graphSnapshot GraphSnapshotCoordinate,
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisRef,
	runtimeCoordinate typedmemory.SHA256Digest,
	outcomes []AssertionOutcome,
) []byte {
	writer := newCanonicalWriter(
		"haft.project-typeenv.existing-assertion-revalidation-report.v1",
	)
	writer.addString(targetTypeEnv.String())
	writer.addBytes(graphSnapshot.CanonicalBytes())
	writer.addString(runtimeBasis.String())
	writer.addString(runtimeCoordinate.String())
	writer.addUint64(uint64(len(outcomes)))
	for _, outcome := range outcomes {
		writer.addBytes(outcome.CanonicalBytes())
	}
	return writer.bytes()
}

func digestBytes(raw []byte) (typedmemory.SHA256Digest, error) {
	digest := canonicalDigest(raw)
	return typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(digest[:]))
}

func sameAssertionIDs(
	left []typedmemory.AssertionID,
	right []typedmemory.AssertionID,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// DecodeCanonicalReport reconstructs and re-derives one complete report. It
// rejects malformed, non-canonical, trailing, or internally inconsistent
// bytes. The runtime registry itself is deliberately not serialized.
func DecodeCanonicalReport(raw []byte) (Report, error) {
	reader, err := newCanonicalReader(
		raw,
		"haft.project-typeenv.existing-assertion-revalidation-report.v1",
	)
	if err != nil {
		return Report{}, err
	}
	targetRaw, err := reader.readString()
	if err != nil {
		return Report{}, err
	}
	target, err := typedmemory.ParseTypeEnvRef(targetRaw)
	if err != nil {
		return Report{}, err
	}
	snapshotBytes, err := reader.readBytes()
	if err != nil {
		return Report{}, err
	}
	snapshot, err := DecodeCanonicalGraphSnapshotCoordinate(snapshotBytes)
	if err != nil {
		return Report{}, err
	}
	runtimeRaw, err := reader.readString()
	if err != nil {
		return Report{}, err
	}
	runtime, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(runtimeRaw)
	if err != nil {
		return Report{}, err
	}
	coordinateRaw, err := reader.readString()
	if err != nil {
		return Report{}, err
	}
	coordinate, err := typedmemory.NewSHA256Digest(coordinateRaw)
	if err != nil {
		return Report{}, err
	}
	count, err := reader.readUint64()
	if err != nil {
		return Report{}, err
	}
	if count > maximumCanonicalElements {
		return Report{}, fmt.Errorf("canonical report outcome count exceeds the supported bound")
	}
	outcomes := make([]AssertionOutcome, 0, count)
	for index := uint64(0); index < count; index++ {
		encoded, readErr := reader.readBytes()
		if readErr != nil {
			return Report{}, readErr
		}
		outcome, decodeErr := decodeCanonicalOutcome(encoded)
		if decodeErr != nil {
			return Report{}, fmt.Errorf(
				"canonical report outcome %d: %w",
				index,
				decodeErr,
			)
		}
		outcomes = append(outcomes, outcome)
	}
	if err := reader.requireEnd(); err != nil {
		return Report{}, err
	}
	report, err := NewReport(target, snapshot, runtime, coordinate, outcomes)
	if err != nil {
		return Report{}, err
	}
	if !bytes.Equal(report.canonicalBytes, raw) {
		return Report{}, fmt.Errorf("revalidation report is not canonical")
	}
	return report, nil
}

func decodeCanonicalOutcome(raw []byte) (AssertionOutcome, error) {
	reader, err := newCanonicalReader(
		raw,
		"haft.project-typeenv.assertion-revalidation-outcome.v1",
	)
	if err != nil {
		return nil, err
	}
	kindRaw, err := reader.readString()
	if err != nil {
		return nil, err
	}
	kind, err := parseOutcomeKind(kindRaw)
	if err != nil {
		return nil, err
	}
	assertionRaw, err := reader.readString()
	if err != nil {
		return nil, err
	}
	assertion, err := typedmemory.NewAssertionID(assertionRaw)
	if err != nil {
		return nil, err
	}
	digestRaw, err := reader.readString()
	if err != nil {
		return nil, err
	}
	relationDigest, err := typedmemory.NewSHA256Digest(digestRaw)
	if err != nil {
		return nil, err
	}
	count, err := reader.readUint64()
	if err != nil {
		return nil, err
	}
	if count > maximumCanonicalElements {
		return nil, fmt.Errorf("canonical outcome ground count exceeds the supported bound")
	}
	grounds := make([]Ground, 0, count)
	for index := uint64(0); index < count; index++ {
		encoded, readErr := reader.readBytes()
		if readErr != nil {
			return nil, readErr
		}
		ground, decodeErr := decodeCanonicalGround(encoded)
		if decodeErr != nil {
			return nil, fmt.Errorf("outcome ground %d: %w", index, decodeErr)
		}
		grounds = append(grounds, ground)
	}
	if err := reader.requireEnd(); err != nil {
		return nil, err
	}
	outcome, err := NewAssertionOutcome(assertion, relationDigest, grounds)
	if err != nil {
		return nil, err
	}
	if outcome.Kind() != kind || !bytes.Equal(outcome.CanonicalBytes(), raw) {
		return nil, fmt.Errorf("assertion outcome is not canonical")
	}
	return outcome, nil
}

func parseOutcomeKind(raw string) (AssertionOutcomeKind, error) {
	for candidate := AssertionValid; candidate <= AssertionUnderdetermined; candidate++ {
		if candidate.String() == raw {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("assertion outcome kind %q is invalid", raw)
}

func decodeCanonicalGround(raw []byte) (Ground, error) {
	reader, err := newCanonicalReader(
		raw,
		"haft.project-typeenv.assertion-revalidation-ground.v1",
	)
	if err != nil {
		return Ground{}, err
	}
	postureRaw, err := reader.readString()
	if err != nil {
		return Ground{}, err
	}
	posture, err := parseGroundPosture(postureRaw)
	if err != nil {
		return Ground{}, err
	}
	code, err := reader.readString()
	if err != nil {
		return Ground{}, err
	}
	path, err := reader.readString()
	if err != nil {
		return Ground{}, err
	}
	message, err := reader.readString()
	if err != nil {
		return Ground{}, err
	}
	repair, err := reader.readString()
	if err != nil {
		return Ground{}, err
	}
	count, err := reader.readUint64()
	if err != nil {
		return Ground{}, err
	}
	if count > maximumCanonicalElements {
		return Ground{}, fmt.Errorf("canonical ground detail count exceeds the supported bound")
	}
	details := make([]GroundDetail, 0, count)
	for index := uint64(0); index < count; index++ {
		key, readErr := reader.readString()
		if readErr != nil {
			return Ground{}, readErr
		}
		valueCount, readErr := reader.readUint64()
		if readErr != nil {
			return Ground{}, readErr
		}
		if valueCount > maximumCanonicalElements {
			return Ground{}, fmt.Errorf(
				"canonical ground detail value count exceeds the supported bound",
			)
		}
		values := make([]string, 0, valueCount)
		for valueIndex := uint64(0); valueIndex < valueCount; valueIndex++ {
			value, valueErr := reader.readString()
			if valueErr != nil {
				return Ground{}, valueErr
			}
			values = append(values, value)
		}
		details = append(details, GroundDetail{key: key, values: values})
	}
	if err := reader.requireEnd(); err != nil {
		return Ground{}, err
	}
	ground, err := newGround(
		posture,
		GroundCode(code),
		path,
		message,
		details,
		repair,
	)
	if err != nil {
		return Ground{}, err
	}
	if !bytes.Equal(ground.CanonicalBytes(), raw) {
		return Ground{}, fmt.Errorf("ground is not canonical")
	}
	return ground, nil
}

func parseGroundPosture(raw string) (GroundPosture, error) {
	for candidate := GroundInvalid; candidate <= GroundMissingBasis; candidate++ {
		if candidate.String() == raw {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("ground posture %q is invalid", raw)
}
