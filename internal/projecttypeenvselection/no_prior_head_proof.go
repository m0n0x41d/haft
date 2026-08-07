package projecttypeenvselection

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	noPriorHeadProofDomain       = "haft.project-typeenv.no-prior-head-proof.v1"
	maximumNoPriorHeadProofBytes = 64 << 10
)

// NoPriorHeadProofRecord is an immutable content-addressed description of one
// exact absence observation. Decoding or verifying this record proves only its
// canonical structure and content identity; it does not prove that storage
// issued the observation or that head absence remains current.
//
// A later effect boundary must reread the stable head slot and mint a separate,
// non-serializable storage witness before it may install a Genesis successor.
type NoPriorHeadProofRecord struct {
	ref                   NoPriorHeadProofRef
	project               projectidentity.ProjectID
	head                  ProjectTypeEnvHeadRef
	graphSnapshot         ProjectGraphSnapshotBasisRef
	graphSnapshotDigest   typedmemory.SHA256Digest
	expectedGraphRevision typedmemory.GraphRevision
	canonicalBytes        []byte
}

// DecodeNoPriorHeadProof authenticates record structure and content identity.
// It does not recreate the transaction-owned observation that issued it.
func DecodeNoPriorHeadProof(canonical []byte) (NoPriorHeadProofRecord, error) {
	if len(canonical) == 0 {
		return NoPriorHeadProofRecord{}, fmt.Errorf("no-prior-head proof is empty")
	}
	if len(canonical) > maximumNoPriorHeadProofBytes {
		return NoPriorHeadProofRecord{}, fmt.Errorf(
			"no-prior-head proof exceeds %d bytes",
			maximumNoPriorHeadProofBytes,
		)
	}
	reader := stageReader{value: canonical}
	domain, err := reader.readString("no-prior-head proof domain")
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	if domain != noPriorHeadProofDomain {
		return NoPriorHeadProofRecord{}, fmt.Errorf("no-prior-head proof domain is invalid")
	}
	state, err := decodeNoPriorHeadProofState(&reader)
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	if reader.remaining() != 0 {
		return NoPriorHeadProofRecord{}, fmt.Errorf("no-prior-head proof has trailing bytes")
	}
	normalized, err := normalizeNoPriorHeadProofState(state)
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	reencoded := encodeNoPriorHeadProofState(normalized)
	if !bytes.Equal(reencoded, canonical) {
		return NoPriorHeadProofRecord{}, fmt.Errorf("no-prior-head proof is not canonical")
	}
	digest, err := deriveStageProjectionDigest(canonical)
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	return NoPriorHeadProofRecord{
		ref:                   NoPriorHeadProofRef{digest: digest},
		project:               normalized.project,
		head:                  normalized.head,
		graphSnapshot:         normalized.graphSnapshot,
		graphSnapshotDigest:   normalized.graphSnapshotDigest,
		expectedGraphRevision: normalized.expectedGraphRevision,
		canonicalBytes:        append([]byte(nil), canonical...),
	}, nil
}

func VerifyNoPriorHeadProof(
	expected NoPriorHeadProofRef,
	canonical []byte,
) (NoPriorHeadProofRecord, error) {
	parsed, err := ParseNoPriorHeadProofRef(expected.String())
	if err != nil || parsed != expected {
		return NoPriorHeadProofRecord{}, fmt.Errorf("expected no-prior-head proof reference is invalid")
	}
	proof, err := DecodeNoPriorHeadProof(canonical)
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	if proof.ref != expected {
		return NoPriorHeadProofRecord{}, fmt.Errorf("no-prior-head proof reference mismatch")
	}
	return proof, nil
}

func (proof NoPriorHeadProofRecord) Ref() NoPriorHeadProofRef { return proof.ref }

func (proof NoPriorHeadProofRecord) Project() projectidentity.ProjectID { return proof.project }

func (proof NoPriorHeadProofRecord) Head() ProjectTypeEnvHeadRef { return proof.head }

func (proof NoPriorHeadProofRecord) GraphSnapshotBasis() ProjectGraphSnapshotBasisRef {
	return proof.graphSnapshot
}

func (proof NoPriorHeadProofRecord) GraphSnapshotBasisDigest() typedmemory.SHA256Digest {
	return proof.graphSnapshotDigest
}

func (proof NoPriorHeadProofRecord) ExpectedGraphRevision() typedmemory.GraphRevision {
	return proof.expectedGraphRevision
}

func (proof NoPriorHeadProofRecord) CanonicalBytes() []byte {
	return append([]byte(nil), proof.canonicalBytes...)
}

func (proof NoPriorHeadProofRecord) Verify() error {
	verified, err := VerifyNoPriorHeadProof(proof.ref, proof.canonicalBytes)
	if err != nil {
		return err
	}
	if verified.project != proof.project ||
		verified.head != proof.head ||
		verified.graphSnapshot != proof.graphSnapshot ||
		verified.graphSnapshotDigest != proof.graphSnapshotDigest ||
		verified.expectedGraphRevision != proof.expectedGraphRevision {
		return fmt.Errorf("no-prior-head proof stored state differs from canonical bytes")
	}
	return nil
}

func VerifyNoPriorHeadProofAgainstGraphSnapshot(
	proof NoPriorHeadProofRecord,
	snapshot ProjectGraphSnapshotBasis,
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
		{proof.graphSnapshotDigest == snapshot.Ref().Digest(), "snapshot digest"},
		{proof.expectedGraphRevision == snapshot.GraphRevision(), "graph revision"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf("no-prior-head proof %s mismatch", check.label)
		}
	}
	return nil
}

type noPriorHeadProofState struct {
	project               projectidentity.ProjectID
	head                  ProjectTypeEnvHeadRef
	graphSnapshot         ProjectGraphSnapshotBasisRef
	graphSnapshotDigest   typedmemory.SHA256Digest
	expectedGraphRevision typedmemory.GraphRevision
}

func normalizeNoPriorHeadProofState(
	state noPriorHeadProofState,
) (noPriorHeadProofState, error) {
	project, err := projectidentity.ParseProjectID(state.project.String())
	if err != nil || project != state.project {
		return noPriorHeadProofState{}, fmt.Errorf("no-prior-head proof project is required")
	}
	head, err := ParseProjectTypeEnvHeadRef(state.head.String())
	if err != nil || head != state.head {
		return noPriorHeadProofState{}, fmt.Errorf("no-prior-head proof head is required")
	}
	expectedHead, err := ProjectTypeEnvHeadRefForProject(project)
	if err != nil || head != expectedHead {
		return noPriorHeadProofState{}, fmt.Errorf("no-prior-head proof head project mismatch")
	}
	snapshot, err := ParseProjectGraphSnapshotBasisRef(state.graphSnapshot.String())
	if err != nil || snapshot != state.graphSnapshot {
		return noPriorHeadProofState{}, fmt.Errorf("no-prior-head proof graph snapshot is required")
	}
	digest, err := typedmemory.NewSHA256Digest(state.graphSnapshotDigest.String())
	if err != nil || digest != state.graphSnapshotDigest || snapshot.Digest() != digest {
		return noPriorHeadProofState{}, fmt.Errorf("no-prior-head proof graph snapshot digest mismatch")
	}
	return noPriorHeadProofState{
		project:               project,
		head:                  head,
		graphSnapshot:         snapshot,
		graphSnapshotDigest:   digest,
		expectedGraphRevision: state.expectedGraphRevision,
	}, nil
}

func encodeNoPriorHeadProofState(state noPriorHeadProofState) []byte {
	writer := stageWriter{}
	writer.addString(noPriorHeadProofDomain)
	writer.addString(state.project.String())
	writer.addString(state.head.String())
	writer.addString(state.graphSnapshot.String())
	writer.addString(state.graphSnapshotDigest.String())
	writer.addUint64(state.expectedGraphRevision.Value())
	return writer.bytes()
}

func decodeNoPriorHeadProofState(reader *stageReader) (noPriorHeadProofState, error) {
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
	head, err := ParseProjectTypeEnvHeadRef(headText)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	snapshotText, err := reader.readString("no-prior-head proof graph snapshot")
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	snapshot, err := ParseProjectGraphSnapshotBasisRef(snapshotText)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	digestText, err := reader.readString("no-prior-head proof graph snapshot digest")
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	revision, err := reader.readUint64("no-prior-head proof expected graph revision")
	if err != nil {
		return noPriorHeadProofState{}, err
	}
	return noPriorHeadProofState{
		project:               project,
		head:                  head,
		graphSnapshot:         snapshot,
		graphSnapshotDigest:   digest,
		expectedGraphRevision: typedmemory.NewGraphRevision(revision),
	}, nil
}
