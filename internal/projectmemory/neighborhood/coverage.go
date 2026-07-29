package neighborhood

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type FacetCoverageKind string

const (
	CoverageComplete      FacetCoverageKind = "complete"
	CoveragePartial       FacetCoverageKind = "partial"
	CoverageNotApplicable FacetCoverageKind = "not_applicable"
	CoverageUnavailable   FacetCoverageKind = "unavailable"
	CoverageStale         FacetCoverageKind = "stale"
)

type ApplicabilityBasisRef struct{ value string }
type MissingBasisRef struct{ value string }
type RetryBasisRef struct{ value string }

func NewApplicabilityBasisRef(raw string) (ApplicabilityBasisRef, error) {
	value, err := exactReference("applicability basis", raw)
	if err != nil {
		return ApplicabilityBasisRef{}, err
	}
	return ApplicabilityBasisRef{value: value}, nil
}

func NewMissingBasisRef(raw string) (MissingBasisRef, error) {
	value, err := exactReference("missing basis", raw)
	if err != nil {
		return MissingBasisRef{}, err
	}
	return MissingBasisRef{value: value}, nil
}

func NewRetryBasisRef(raw string) (RetryBasisRef, error) {
	value, err := exactReference("retry basis", raw)
	if err != nil {
		return RetryBasisRef{}, err
	}
	return RetryBasisRef{value: value}, nil
}

func (ref ApplicabilityBasisRef) String() string { return ref.value }
func (ref MissingBasisRef) String() string       { return ref.value }
func (ref RetryBasisRef) String() string         { return ref.value }

// SnapshotCursor is bound to one graph/TypeEnv/profile/facet coordinate.
// Changing any coordinate creates a different cursor digest.
type SnapshotCursor struct {
	graphRevision typedmemory.GraphRevision
	typeEnv       typedmemory.TypeEnvRef
	profile       ProjectionProfileRef
	profileDigest typedmemory.SHA256Digest
	facet         FacetKind
	nextOffset    uint64
	digest        typedmemory.SHA256Digest
}

func NewSnapshotCursor(
	graphRevision typedmemory.GraphRevision,
	typeEnv typedmemory.TypeEnvRef,
	profile ProjectionProfileDefinition,
	facet FacetKind,
	nextOffset uint64,
) (SnapshotCursor, error) {
	cursor := SnapshotCursor{
		graphRevision: graphRevision,
		typeEnv:       typeEnv,
		profile:       profile.Ref(),
		profileDigest: profile.Digest(),
		facet:         facet,
		nextOffset:    nextOffset,
	}
	digest, err := snapshotCursorDigest(cursor)
	if err != nil {
		return SnapshotCursor{}, err
	}
	cursor.digest = digest
	if !cursor.Valid() {
		return SnapshotCursor{}, fmt.Errorf("snapshot cursor is invalid")
	}
	return cursor, nil
}

func (cursor SnapshotCursor) Digest() typedmemory.SHA256Digest {
	return cursor.digest
}

func (cursor SnapshotCursor) GraphRevision() typedmemory.GraphRevision {
	return cursor.graphRevision
}

func (cursor SnapshotCursor) TypeEnv() typedmemory.TypeEnvRef {
	return cursor.typeEnv
}

func (cursor SnapshotCursor) ProfileRef() ProjectionProfileRef {
	return cursor.profile
}

func (cursor SnapshotCursor) Facet() FacetKind {
	return cursor.facet
}

func (cursor SnapshotCursor) NextOffset() uint64 {
	return cursor.nextOffset
}

func (cursor SnapshotCursor) Valid() bool {
	if cursor.graphRevision.Value() == 0 ||
		cursor.typeEnv.String() == "" ||
		!cursor.facet.Valid() ||
		cursor.nextOffset == 0 {
		return false
	}
	profile, found := LookupProjectionProfile(cursor.profile)
	if !found ||
		profile.Digest() != cursor.profileDigest ||
		!profile.AllowsFacet(cursor.facet) {
		return false
	}
	digest, err := snapshotCursorDigest(cursor)
	return err == nil && digest == cursor.digest
}

type snapshotCursorCanonicalV1 struct {
	GraphRevision uint64 `json:"graph_revision"`
	TypeEnv       string `json:"type_env_ref"`
	Profile       string `json:"profile_ref"`
	ProfileDigest string `json:"profile_digest"`
	Facet         string `json:"facet"`
	NextOffset    uint64 `json:"next_offset"`
}

func snapshotCursorDigest(
	cursor SnapshotCursor,
) (typedmemory.SHA256Digest, error) {
	carrier := snapshotCursorCanonicalV1{
		GraphRevision: cursor.graphRevision.Value(),
		TypeEnv:       cursor.typeEnv.String(),
		Profile:       cursor.profile.String(),
		ProfileDigest: cursor.profileDigest.String(),
		Facet:         string(cursor.facet),
		NextOffset:    cursor.nextOffset,
	}
	canonical, err := json.Marshal(carrier)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf(
			"encode snapshot cursor: %w",
			err,
		)
	}
	sum := sha256.Sum256(canonical)
	raw := "sha256:" + hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest(raw)
}

// FacetCoverage is a closed union. Empty Complete is known empty at the exact
// snapshot and cannot be confused with unavailable, stale, or not applicable.
type FacetCoverage interface {
	Kind() FacetCoverageKind
	Included() uint64
	isFacetCoverage()
}

type CompleteCoverage struct {
	included uint64
}

func NewCompleteCoverage(included uint64) CompleteCoverage {
	return CompleteCoverage{included: included}
}

func (coverage CompleteCoverage) Kind() FacetCoverageKind {
	return CoverageComplete
}

func (coverage CompleteCoverage) Included() uint64 {
	return coverage.included
}

func (CompleteCoverage) isFacetCoverage() {}

type PartialCoverage struct {
	included       uint64
	omittedAtLeast uint64
	cursor         SnapshotCursor
}

func NewPartialCoverage(
	included uint64,
	omittedAtLeast uint64,
	cursor SnapshotCursor,
) (PartialCoverage, error) {
	coverage := PartialCoverage{
		included:       included,
		omittedAtLeast: omittedAtLeast,
		cursor:         cursor,
	}
	if omittedAtLeast == 0 || !cursor.Valid() {
		return PartialCoverage{}, fmt.Errorf(
			"partial coverage requires omitted items and an exact cursor",
		)
	}
	return coverage, nil
}

func (coverage PartialCoverage) Kind() FacetCoverageKind {
	return CoveragePartial
}

func (coverage PartialCoverage) Included() uint64 {
	return coverage.included
}

func (coverage PartialCoverage) OmittedAtLeast() uint64 {
	return coverage.omittedAtLeast
}

func (coverage PartialCoverage) Cursor() SnapshotCursor {
	return coverage.cursor
}

func (PartialCoverage) isFacetCoverage() {}

type NotApplicableCoverage struct {
	basis ApplicabilityBasisRef
}

func NewNotApplicableCoverage(
	basis ApplicabilityBasisRef,
) (NotApplicableCoverage, error) {
	if basis.String() == "" {
		return NotApplicableCoverage{}, fmt.Errorf(
			"not-applicable coverage requires exact basis",
		)
	}
	return NotApplicableCoverage{basis: basis}, nil
}

func (coverage NotApplicableCoverage) Kind() FacetCoverageKind {
	return CoverageNotApplicable
}

func (NotApplicableCoverage) Included() uint64 {
	return 0
}

func (coverage NotApplicableCoverage) Basis() ApplicabilityBasisRef {
	return coverage.basis
}

func (NotApplicableCoverage) isFacetCoverage() {}

type UnavailableCoverage struct {
	missing MissingBasisRef
}

func NewUnavailableCoverage(
	missing MissingBasisRef,
) (UnavailableCoverage, error) {
	if missing.String() == "" {
		return UnavailableCoverage{}, fmt.Errorf(
			"unavailable coverage requires typed missing basis",
		)
	}
	return UnavailableCoverage{missing: missing}, nil
}

func (coverage UnavailableCoverage) Kind() FacetCoverageKind {
	return CoverageUnavailable
}

func (UnavailableCoverage) Included() uint64 {
	return 0
}

func (coverage UnavailableCoverage) MissingBasis() MissingBasisRef {
	return coverage.missing
}

func (UnavailableCoverage) isFacetCoverage() {}

type StaleCoverage struct {
	retry RetryBasisRef
}

func NewStaleCoverage(
	retry RetryBasisRef,
) (StaleCoverage, error) {
	if retry.String() == "" {
		return StaleCoverage{}, fmt.Errorf(
			"stale coverage requires typed retry basis",
		)
	}
	return StaleCoverage{retry: retry}, nil
}

func (coverage StaleCoverage) Kind() FacetCoverageKind {
	return CoverageStale
}

func (StaleCoverage) Included() uint64 {
	return 0
}

func (coverage StaleCoverage) RetryBasis() RetryBasisRef {
	return coverage.retry
}

func (StaleCoverage) isFacetCoverage() {}

func exactReference(label string, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value != raw || strings.ContainsAny(value, "\r\n\t") {
		return "", fmt.Errorf("%s reference is not exact", label)
	}
	return value, nil
}
