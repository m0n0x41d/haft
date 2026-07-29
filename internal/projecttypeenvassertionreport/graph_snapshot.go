package projecttypeenvassertionreport

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const graphSnapshotRefPrefix = "project-graph-snapshot-basis:"

// GraphSnapshotRef is the lower-layer identity of one exact project graph
// snapshot basis. It deliberately mirrors only the stable digest coordinate,
// not the selection package's graph-closure representation.
type GraphSnapshotRef struct {
	digest typedmemory.SHA256Digest
}

func ParseGraphSnapshotRef(raw string) (GraphSnapshotRef, error) {
	digestRaw, found := strings.CutPrefix(raw, graphSnapshotRefPrefix)
	if !found {
		return GraphSnapshotRef{}, fmt.Errorf(
			"graph snapshot reference is malformed",
		)
	}
	digest, err := typedmemory.NewSHA256Digest(digestRaw)
	if err != nil {
		return GraphSnapshotRef{}, fmt.Errorf(
			"graph snapshot reference: %w",
			err,
		)
	}
	ref := GraphSnapshotRef{digest: digest}
	if ref.String() != raw {
		return GraphSnapshotRef{}, fmt.Errorf(
			"graph snapshot reference is not canonical",
		)
	}
	return ref, nil
}

func (ref GraphSnapshotRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref GraphSnapshotRef) String() string {
	return graphSnapshotRefPrefix + ref.digest.String()
}

// GraphSnapshotCoordinate binds the exact graph-basis reference, its content
// digest, and its observed revision without importing the upper selection
// package. The reference digest and explicit basis digest must agree.
type GraphSnapshotCoordinate struct {
	ref            GraphSnapshotRef
	revision       typedmemory.GraphRevision
	basisDigest    typedmemory.SHA256Digest
	canonicalBytes []byte
}

func NewGraphSnapshotCoordinate(
	ref GraphSnapshotRef,
	revision typedmemory.GraphRevision,
	basisDigest typedmemory.SHA256Digest,
) (GraphSnapshotCoordinate, error) {
	canonicalRef, err := ParseGraphSnapshotRef(ref.String())
	if err != nil || canonicalRef != ref {
		return GraphSnapshotCoordinate{}, fmt.Errorf(
			"graph snapshot coordinate requires an exact reference",
		)
	}
	canonicalDigest, err := typedmemory.NewSHA256Digest(
		basisDigest.String(),
	)
	if err != nil || canonicalDigest != basisDigest {
		return GraphSnapshotCoordinate{}, fmt.Errorf(
			"graph snapshot coordinate requires an exact basis digest",
		)
	}
	if canonicalRef.Digest() != canonicalDigest {
		return GraphSnapshotCoordinate{}, fmt.Errorf(
			"graph snapshot reference and basis digest differ",
		)
	}
	canonical := canonicalGraphSnapshotCoordinate(
		canonicalRef,
		revision,
		canonicalDigest,
	)
	return GraphSnapshotCoordinate{
		ref:            canonicalRef,
		revision:       revision,
		basisDigest:    canonicalDigest,
		canonicalBytes: canonical,
	}, nil
}

func (coordinate GraphSnapshotCoordinate) Ref() GraphSnapshotRef {
	return coordinate.ref
}

func (coordinate GraphSnapshotCoordinate) Revision() typedmemory.GraphRevision {
	return coordinate.revision
}

func (coordinate GraphSnapshotCoordinate) BasisDigest() typedmemory.SHA256Digest {
	return coordinate.basisDigest
}

func (coordinate GraphSnapshotCoordinate) CanonicalBytes() []byte {
	return append([]byte(nil), coordinate.canonicalBytes...)
}

func (coordinate GraphSnapshotCoordinate) Verify() error {
	rebuilt, err := NewGraphSnapshotCoordinate(
		coordinate.ref,
		coordinate.revision,
		coordinate.basisDigest,
	)
	if err != nil {
		return err
	}
	if !bytes.Equal(rebuilt.canonicalBytes, coordinate.canonicalBytes) {
		return fmt.Errorf("graph snapshot coordinate state is inconsistent")
	}
	return nil
}

func DecodeCanonicalGraphSnapshotCoordinate(
	raw []byte,
) (GraphSnapshotCoordinate, error) {
	reader, err := newCanonicalReader(
		raw,
		"haft.project-typeenv.assertion-revalidation-graph-snapshot.v1",
	)
	if err != nil {
		return GraphSnapshotCoordinate{}, err
	}
	refRaw, err := reader.readString()
	if err != nil {
		return GraphSnapshotCoordinate{}, err
	}
	ref, err := ParseGraphSnapshotRef(refRaw)
	if err != nil {
		return GraphSnapshotCoordinate{}, err
	}
	revision, err := reader.readUint64()
	if err != nil {
		return GraphSnapshotCoordinate{}, err
	}
	digestRaw, err := reader.readString()
	if err != nil {
		return GraphSnapshotCoordinate{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(digestRaw)
	if err != nil {
		return GraphSnapshotCoordinate{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return GraphSnapshotCoordinate{}, err
	}
	coordinate, err := NewGraphSnapshotCoordinate(
		ref,
		typedmemory.NewGraphRevision(revision),
		digest,
	)
	if err != nil {
		return GraphSnapshotCoordinate{}, err
	}
	if !bytes.Equal(coordinate.canonicalBytes, raw) {
		return GraphSnapshotCoordinate{}, fmt.Errorf(
			"graph snapshot coordinate is not canonical",
		)
	}
	return coordinate, nil
}

func canonicalGraphSnapshotCoordinate(
	ref GraphSnapshotRef,
	revision typedmemory.GraphRevision,
	basisDigest typedmemory.SHA256Digest,
) []byte {
	writer := newCanonicalWriter(
		"haft.project-typeenv.assertion-revalidation-graph-snapshot.v1",
	)
	writer.addString(ref.String())
	writer.addUint64(revision.Value())
	writer.addString(basisDigest.String())
	return writer.bytes()
}
