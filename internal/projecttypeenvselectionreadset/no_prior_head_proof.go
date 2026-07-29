package projecttypeenvselectionreadset

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"time"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	noPriorHeadProofDomain       = "haft.project-typeenv.no-prior-head-proof.v2"
	noPriorHeadProofRefPrefix    = "project-typeenv-no-prior-head-proof:"
	maximumNoPriorHeadProofBytes = 64 << 10
	maximumNoPriorHeadProofText  = 16 << 10
)

// NoPriorHeadProofRecord is an immutable content-addressed audit record of one
// exact dedicated-head absence observed through an active BEGIN IMMEDIATE
// transaction. Decoding verifies canonical structure and content identity; it
// does not recreate the transaction-local capability that issued the record.
//
// This package intentionally exports no sealer or issuer. Production issuance
// is private to observeAbsentHead after the exact storage read reports absence.
type NoPriorHeadProofRecord struct {
	ref                 projecttypeenvselection.NoPriorHeadProofRef
	project             projectidentity.ProjectID
	head                projecttypeenvselection.ProjectTypeEnvHeadRef
	graphSnapshot       projecttypeenvselection.ProjectGraphSnapshotBasisRef
	graphSnapshotDigest typedmemory.SHA256Digest
	graphRevision       typedmemory.GraphRevision
	observedAt          time.Time
	canonicalBytes      []byte
}

type noPriorHeadProofInput struct {
	project      projectidentity.ProjectID
	head         projecttypeenvselection.ProjectTypeEnvHeadRef
	currentGraph projecttypeenvselection.ProjectGraphSnapshotBasis
	observedAt   time.Time
}

type noPriorHeadProofState struct {
	project             projectidentity.ProjectID
	head                projecttypeenvselection.ProjectTypeEnvHeadRef
	graphSnapshot       projecttypeenvselection.ProjectGraphSnapshotBasisRef
	graphSnapshotDigest typedmemory.SHA256Digest
	graphRevision       typedmemory.GraphRevision
	observedAt          time.Time
}

func sealNoPriorHeadProofRecord(
	input noPriorHeadProofInput,
) (NoPriorHeadProofRecord, error) {
	if err := input.currentGraph.Verify(); err != nil {
		return NoPriorHeadProofRecord{}, fmt.Errorf(
			"verify no-prior-head current graph: %w",
			err,
		)
	}
	state, err := normalizeNoPriorHeadProofState(noPriorHeadProofState{
		project:             input.project,
		head:                input.head,
		graphSnapshot:       input.currentGraph.Ref(),
		graphSnapshotDigest: input.currentGraph.Ref().Digest(),
		graphRevision:       input.currentGraph.GraphRevision(),
		observedAt:          input.observedAt,
	})
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	checks := []struct {
		matches bool
		label   string
	}{
		{input.currentGraph.Project() == state.project, "project"},
		{
			input.currentGraph.GraphRevision() == state.graphRevision,
			"graph revision",
		},
	}
	for _, check := range checks {
		if !check.matches {
			return NoPriorHeadProofRecord{}, fmt.Errorf(
				"no-prior-head current graph %s mismatch",
				check.label,
			)
		}
	}
	canonical := encodeNoPriorHeadProofState(state)
	return DecodeNoPriorHeadProof(canonical)
}

// DecodeNoPriorHeadProof authenticates canonical structure and content
// identity. It does not prove that the record was issued by a live transaction
// or that head absence remains current.
func DecodeNoPriorHeadProof(
	canonical []byte,
) (NoPriorHeadProofRecord, error) {
	reader, err := newNoPriorHeadProofReader(canonical)
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	state, err := decodeNoPriorHeadProofState(reader)
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	if reader.remaining() != 0 {
		return NoPriorHeadProofRecord{}, fmt.Errorf(
			"no-prior-head proof has trailing bytes",
		)
	}
	normalized, err := normalizeNoPriorHeadProofState(state)
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	reencoded := encodeNoPriorHeadProofState(normalized)
	if !bytes.Equal(reencoded, canonical) {
		return NoPriorHeadProofRecord{}, fmt.Errorf(
			"no-prior-head proof is not canonical",
		)
	}
	ref, err := noPriorHeadProofRefForCanonical(canonical)
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	return NoPriorHeadProofRecord{
		ref:                 ref,
		project:             normalized.project,
		head:                normalized.head,
		graphSnapshot:       normalized.graphSnapshot,
		graphSnapshotDigest: normalized.graphSnapshotDigest,
		graphRevision:       normalized.graphRevision,
		observedAt:          normalized.observedAt,
		canonicalBytes:      append([]byte(nil), canonical...),
	}, nil
}

// VerifyNoPriorHeadProof authenticates canonical bytes against an expected
// content address. It does not recreate transaction-local issuance authority.
func VerifyNoPriorHeadProof(
	expected projecttypeenvselection.NoPriorHeadProofRef,
	canonical []byte,
) (NoPriorHeadProofRecord, error) {
	parsed, err := projecttypeenvselection.ParseNoPriorHeadProofRef(
		expected.String(),
	)
	if err != nil || parsed != expected {
		return NoPriorHeadProofRecord{}, fmt.Errorf(
			"expected no-prior-head proof reference is invalid",
		)
	}
	proof, err := DecodeNoPriorHeadProof(canonical)
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	if proof.ref != expected {
		return NoPriorHeadProofRecord{}, fmt.Errorf(
			"no-prior-head proof reference mismatch",
		)
	}
	return proof, nil
}

func (proof NoPriorHeadProofRecord) Ref() projecttypeenvselection.NoPriorHeadProofRef {
	return proof.ref
}

func (proof NoPriorHeadProofRecord) Digest() typedmemory.SHA256Digest {
	return proof.ref.Digest()
}

func (proof NoPriorHeadProofRecord) Project() projectidentity.ProjectID {
	return proof.project
}

func (proof NoPriorHeadProofRecord) Head() projecttypeenvselection.ProjectTypeEnvHeadRef {
	return proof.head
}

func (proof NoPriorHeadProofRecord) GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasisRef {
	return proof.graphSnapshot
}

func (proof NoPriorHeadProofRecord) GraphSnapshotBasisDigest() typedmemory.SHA256Digest {
	return proof.graphSnapshotDigest
}

func (proof NoPriorHeadProofRecord) GraphRevision() typedmemory.GraphRevision {
	return proof.graphRevision
}

func (proof NoPriorHeadProofRecord) ObservedAt() time.Time {
	return proof.observedAt
}

func (proof NoPriorHeadProofRecord) CanonicalBytes() []byte {
	return append([]byte(nil), proof.canonicalBytes...)
}

func (proof NoPriorHeadProofRecord) Verify() error {
	verified, err := VerifyNoPriorHeadProof(proof.ref, proof.canonicalBytes)
	if err != nil {
		return err
	}
	checks := []bool{
		verified.project == proof.project,
		verified.head == proof.head,
		verified.graphSnapshot == proof.graphSnapshot,
		verified.graphSnapshotDigest == proof.graphSnapshotDigest,
		verified.graphRevision == proof.graphRevision,
		verified.observedAt.Equal(proof.observedAt),
	}
	for _, matches := range checks {
		if !matches {
			return fmt.Errorf(
				"no-prior-head proof stored state differs from canonical bytes",
			)
		}
	}
	return nil
}

func VerifyNoPriorHeadProofAgainstGraphSnapshot(
	proof NoPriorHeadProofRecord,
	snapshot projecttypeenvselection.ProjectGraphSnapshotBasis,
) error {
	if err := proof.Verify(); err != nil {
		return err
	}
	if err := snapshot.Verify(); err != nil {
		return err
	}
	checks := []struct {
		matches bool
		label   string
	}{
		{proof.project == snapshot.Project(), "project"},
		{proof.graphSnapshot == snapshot.Ref(), "snapshot reference"},
		{
			proof.graphSnapshotDigest == snapshot.Ref().Digest(),
			"snapshot digest",
		},
		{proof.graphRevision == snapshot.GraphRevision(), "graph revision"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf(
				"no-prior-head proof %s mismatch",
				check.label,
			)
		}
	}
	return nil
}

func normalizeNoPriorHeadProofState(
	state noPriorHeadProofState,
) (noPriorHeadProofState, error) {
	project, err := projectidentity.ParseProjectID(state.project.String())
	if err != nil || project != state.project {
		return noPriorHeadProofState{}, fmt.Errorf(
			"no-prior-head proof project is required",
		)
	}
	head, err := projecttypeenvselection.ParseProjectTypeEnvHeadRef(
		state.head.String(),
	)
	if err != nil || head != state.head {
		return noPriorHeadProofState{}, fmt.Errorf(
			"no-prior-head proof head is required",
		)
	}
	expectedHead, err := projecttypeenvselection.ProjectTypeEnvHeadRefForProject(
		project,
	)
	if err != nil || head != expectedHead {
		return noPriorHeadProofState{}, fmt.Errorf(
			"no-prior-head proof head project mismatch",
		)
	}
	snapshot, err := projecttypeenvselection.ParseProjectGraphSnapshotBasisRef(
		state.graphSnapshot.String(),
	)
	if err != nil || snapshot != state.graphSnapshot {
		return noPriorHeadProofState{}, fmt.Errorf(
			"no-prior-head proof graph snapshot is required",
		)
	}
	digest, err := typedmemory.NewSHA256Digest(
		state.graphSnapshotDigest.String(),
	)
	if err != nil ||
		digest != state.graphSnapshotDigest ||
		snapshot.Digest() != digest {
		return noPriorHeadProofState{}, fmt.Errorf(
			"no-prior-head proof graph snapshot digest mismatch",
		)
	}
	observedAt := canonicalNoPriorHeadProofTime(state.observedAt)
	if observedAt.IsZero() {
		return noPriorHeadProofState{}, fmt.Errorf(
			"no-prior-head proof observed_at is required",
		)
	}
	return noPriorHeadProofState{
		project:             project,
		head:                head,
		graphSnapshot:       snapshot,
		graphSnapshotDigest: digest,
		graphRevision:       state.graphRevision,
		observedAt:          observedAt,
	}, nil
}

func encodeNoPriorHeadProofState(state noPriorHeadProofState) []byte {
	writer := noPriorHeadProofWriter{}
	writer.addString(noPriorHeadProofDomain)
	writer.addString(state.project.String())
	writer.addString(state.head.String())
	writer.addString(state.graphSnapshot.String())
	writer.addString(state.graphSnapshotDigest.String())
	writer.addUint64(state.graphRevision.Value())
	writer.addString(formatNoPriorHeadProofTime(state.observedAt))
	return writer.bytes()
}

func decodeNoPriorHeadProofState(
	reader *noPriorHeadProofReader,
) (noPriorHeadProofState, error) {
	projectText, err := reader.readString("no-prior-head proof project")
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	headText, err := reader.readString("no-prior-head proof head")
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	head, err := projecttypeenvselection.ParseProjectTypeEnvHeadRef(headText)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	snapshotText, err := reader.readString(
		"no-prior-head proof graph snapshot",
	)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	snapshot, err := projecttypeenvselection.ParseProjectGraphSnapshotBasisRef(
		snapshotText,
	)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	digestText, err := reader.readString(
		"no-prior-head proof graph snapshot digest",
	)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	revision, err := reader.readUint64(
		"no-prior-head proof graph revision",
	)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	observedAtText, err := reader.readString(
		"no-prior-head proof observed_at",
	)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	observedAt, err := parseNoPriorHeadProofTime(observedAtText)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	return noPriorHeadProofState{
		project:             project,
		head:                head,
		graphSnapshot:       snapshot,
		graphSnapshotDigest: digest,
		graphRevision:       typedmemory.NewGraphRevision(revision),
		observedAt:          observedAt,
	}, nil
}

func noPriorHeadProofRefForCanonical(
	canonical []byte,
) (projecttypeenvselection.NoPriorHeadProofRef, error) {
	sum := sha256.Sum256(canonical)
	encoded := hex.EncodeToString(sum[:])
	digestText := "sha256:" + encoded
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return projecttypeenvselection.NoPriorHeadProofRef{}, err
	}
	refText := noPriorHeadProofRefPrefix + digest.String()
	return projecttypeenvselection.ParseNoPriorHeadProofRef(refText)
}

func canonicalNoPriorHeadProofTime(value time.Time) time.Time {
	return value.Round(0).UTC()
}

func formatNoPriorHeadProofTime(value time.Time) string {
	canonical := canonicalNoPriorHeadProofTime(value)
	return canonical.Format(time.RFC3339Nano)
}

func parseNoPriorHeadProofTime(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"parse no-prior-head proof observed_at: %w",
			err,
		)
	}
	formatted := formatNoPriorHeadProofTime(parsed)
	if raw != formatted || parsed.IsZero() {
		return time.Time{}, fmt.Errorf(
			"no-prior-head proof observed_at is not canonical UTC",
		)
	}
	return parsed, nil
}

type noPriorHeadProofWriter struct {
	value []byte
}

func (writer *noPriorHeadProofWriter) addString(value string) {
	encoded := []byte(value)
	writer.addBytes(encoded)
}

func (writer *noPriorHeadProofWriter) addBytes(value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	writer.value = append(writer.value, length[:]...)
	writer.value = append(writer.value, value...)
}

func (writer *noPriorHeadProofWriter) addUint64(value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.value = append(writer.value, encoded[:]...)
}

func (writer noPriorHeadProofWriter) bytes() []byte {
	return append([]byte(nil), writer.value...)
}

type noPriorHeadProofReader struct {
	value  []byte
	offset int
}

func newNoPriorHeadProofReader(
	canonical []byte,
) (*noPriorHeadProofReader, error) {
	if len(canonical) == 0 {
		return nil, fmt.Errorf("no-prior-head proof is empty")
	}
	if len(canonical) > maximumNoPriorHeadProofBytes {
		return nil, fmt.Errorf(
			"no-prior-head proof exceeds %d bytes",
			maximumNoPriorHeadProofBytes,
		)
	}
	reader := &noPriorHeadProofReader{value: canonical}
	domain, err := reader.readString("no-prior-head proof domain")
	if err != nil {
		return nil, err
	}
	if domain != noPriorHeadProofDomain {
		return nil, fmt.Errorf("no-prior-head proof domain is invalid")
	}
	return reader, nil
}

func (reader *noPriorHeadProofReader) readString(
	name string,
) (string, error) {
	encoded, err := reader.readBytes(name)
	if err != nil {
		return "", err
	}
	if len(encoded) > maximumNoPriorHeadProofText {
		return "", fmt.Errorf(
			"%s exceeds %d bytes",
			name,
			maximumNoPriorHeadProofText,
		)
	}
	if !utf8.Valid(encoded) {
		return "", fmt.Errorf("%s is not valid UTF-8", name)
	}
	return string(encoded), nil
}

func (reader *noPriorHeadProofReader) readBytes(
	name string,
) ([]byte, error) {
	length, err := reader.readUint64(name + " length")
	if err != nil {
		return nil, err
	}
	if length > maximumNoPriorHeadProofBytes {
		return nil, fmt.Errorf("%s exceeds canonical record limit", name)
	}
	lengthValue, exact := noPriorHeadProofSliceIndex(length)
	if !exact {
		return nil, fmt.Errorf("%s length does not fit this runtime", name)
	}
	remaining := reader.remaining()
	if lengthValue > remaining {
		return nil, fmt.Errorf("%s is truncated", name)
	}
	start := reader.offset
	reader.offset += lengthValue
	return append([]byte(nil), reader.value[start:reader.offset]...), nil
}

func noPriorHeadProofSliceIndex(value uint64) (int, bool) {
	if value > math.MaxInt {
		return 0, false
	}
	return int(value), true
}

func (reader *noPriorHeadProofReader) readUint64(
	name string,
) (uint64, error) {
	if reader.remaining() < 8 {
		return 0, fmt.Errorf("%s is truncated", name)
	}
	start := reader.offset
	reader.offset += 8
	return binary.BigEndian.Uint64(reader.value[start:reader.offset]), nil
}

func (reader *noPriorHeadProofReader) remaining() int {
	return len(reader.value) - reader.offset
}
