// Package projecttypeenvselection owns pure, immutable values used to stage
// and select one exact project TypeEnv. It performs no storage, authority, or
// project-head mutation.
package projecttypeenvselection

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	projectGraphSnapshotBasisDomain = "haft.project-typeenv.graph-snapshot-basis.v1"
	projectGraphSnapshotBasisPrefix = "project-graph-snapshot-basis:"

	ProjectGraphMaterializationSchemaV1 = "typed-memory-materialization-closure.v1"

	maximumProjectGraphSnapshotBasisBytes = 64 << 10
	maximumProjectGraphCoordinateBytes    = 4 << 10
)

var graphClosureRefPattern = regexp.MustCompile(
	`^(typed-memory-event|typed-memory-commit):[0-9a-f]{64}$`,
)

// GraphEventRef and GraphCommitRef are deliberately distinct even though the
// current storage projection uses the same digest-shaped grammar.
type GraphEventRef struct{ value string }

func ParseGraphEventRef(raw string) (GraphEventRef, error) {
	if !graphClosureRefPattern.MatchString(raw) || !strings.HasPrefix(raw, "typed-memory-event:") {
		return GraphEventRef{}, fmt.Errorf("graph event reference is malformed")
	}
	return GraphEventRef{value: raw}, nil
}

func (ref GraphEventRef) String() string { return ref.value }

type GraphCommitRef struct{ value string }

func ParseGraphCommitRef(raw string) (GraphCommitRef, error) {
	if !graphClosureRefPattern.MatchString(raw) || !strings.HasPrefix(raw, "typed-memory-commit:") {
		return GraphCommitRef{}, fmt.Errorf("graph commit reference is malformed")
	}
	return GraphCommitRef{value: raw}, nil
}

func (ref GraphCommitRef) String() string { return ref.value }

// ProjectGraphClosure is a closed sum. Revision zero cannot be represented
// with event/commit coordinates, while a committed revision cannot omit them.
type ProjectGraphClosure interface {
	projectGraphClosureVariant()
}

type EmptyProjectGraphClosure struct{}

func (EmptyProjectGraphClosure) projectGraphClosureVariant() {}

type CommittedProjectGraphClosure struct {
	event                 GraphEventRef
	commit                GraphCommitRef
	materializationDigest typedmemory.SHA256Digest
	materializationSchema string
}

type CommittedProjectGraphClosureInput struct {
	Event                 GraphEventRef
	Commit                GraphCommitRef
	MaterializationDigest typedmemory.SHA256Digest
}

func NewCommittedProjectGraphClosure(
	input CommittedProjectGraphClosureInput,
) (CommittedProjectGraphClosure, error) {
	event, err := ParseGraphEventRef(input.Event.String())
	if err != nil || event != input.Event {
		return CommittedProjectGraphClosure{}, fmt.Errorf("committed graph event reference is required")
	}
	commit, err := ParseGraphCommitRef(input.Commit.String())
	if err != nil || commit != input.Commit {
		return CommittedProjectGraphClosure{}, fmt.Errorf("committed graph commit reference is required")
	}
	digest, err := typedmemory.NewSHA256Digest(input.MaterializationDigest.String())
	if err != nil || digest != input.MaterializationDigest {
		return CommittedProjectGraphClosure{}, fmt.Errorf("committed graph materialization digest is required")
	}
	return CommittedProjectGraphClosure{
		event:                 event,
		commit:                commit,
		materializationDigest: digest,
		materializationSchema: ProjectGraphMaterializationSchemaV1,
	}, nil
}

func (closure CommittedProjectGraphClosure) Event() GraphEventRef { return closure.event }

func (closure CommittedProjectGraphClosure) Commit() GraphCommitRef { return closure.commit }

func (closure CommittedProjectGraphClosure) MaterializationDigest() typedmemory.SHA256Digest {
	return closure.materializationDigest
}

func (closure CommittedProjectGraphClosure) MaterializationSchema() string {
	return closure.materializationSchema
}

func (CommittedProjectGraphClosure) projectGraphClosureVariant() {}

type ProjectGraphSnapshotBasisRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectGraphSnapshotBasisRef(raw string) (ProjectGraphSnapshotBasisRef, error) {
	digestText, found := strings.CutPrefix(raw, projectGraphSnapshotBasisPrefix)
	if !found {
		return ProjectGraphSnapshotBasisRef{}, fmt.Errorf("project graph snapshot basis reference is malformed")
	}
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return ProjectGraphSnapshotBasisRef{}, fmt.Errorf("project graph snapshot basis reference: %w", err)
	}
	ref := ProjectGraphSnapshotBasisRef{digest: digest}
	if ref.String() != raw {
		return ProjectGraphSnapshotBasisRef{}, fmt.Errorf("project graph snapshot basis reference is not canonical")
	}
	return ref, nil
}

func (ref ProjectGraphSnapshotBasisRef) Digest() typedmemory.SHA256Digest { return ref.digest }

func (ref ProjectGraphSnapshotBasisRef) String() string {
	return projectGraphSnapshotBasisPrefix + ref.digest.String()
}

// ProjectGraphSnapshotBasis identifies one exact coherent graph read. It is
// not a capability, Stage, head, or proof that the project remains unchanged.
type ProjectGraphSnapshotBasis struct {
	ref            ProjectGraphSnapshotBasisRef
	project        projectidentity.ProjectID
	revision       typedmemory.GraphRevision
	closure        ProjectGraphClosure
	canonicalBytes []byte
}

type ProjectGraphSnapshotBasisInput struct {
	Project       projectidentity.ProjectID
	GraphRevision typedmemory.GraphRevision
	Closure       ProjectGraphClosure
}

func SealProjectGraphSnapshotBasis(
	input ProjectGraphSnapshotBasisInput,
) (ProjectGraphSnapshotBasis, error) {
	project, err := projectidentity.ParseProjectID(input.Project.String())
	if err != nil || project != input.Project {
		return ProjectGraphSnapshotBasis{}, fmt.Errorf("project graph snapshot project is required")
	}
	closure, err := normalizeProjectGraphClosure(input.GraphRevision, input.Closure)
	if err != nil {
		return ProjectGraphSnapshotBasis{}, err
	}
	canonical, err := encodeProjectGraphSnapshotBasis(
		project,
		input.GraphRevision,
		closure,
	)
	if err != nil {
		return ProjectGraphSnapshotBasis{}, err
	}
	return DecodeProjectGraphSnapshotBasis(canonical)
}

func DecodeProjectGraphSnapshotBasis(
	canonical []byte,
) (ProjectGraphSnapshotBasis, error) {
	if len(canonical) == 0 {
		return ProjectGraphSnapshotBasis{}, fmt.Errorf("project graph snapshot basis is empty")
	}
	if len(canonical) > maximumProjectGraphSnapshotBasisBytes {
		return ProjectGraphSnapshotBasis{}, fmt.Errorf(
			"project graph snapshot basis exceeds %d bytes",
			maximumProjectGraphSnapshotBasisBytes,
		)
	}
	reader := graphSnapshotReader{value: canonical}
	domain, err := reader.readString("domain")
	if err != nil {
		return ProjectGraphSnapshotBasis{}, err
	}
	if domain != projectGraphSnapshotBasisDomain {
		return ProjectGraphSnapshotBasis{}, fmt.Errorf("project graph snapshot basis domain is invalid")
	}
	projectText, err := reader.readString("project")
	if err != nil {
		return ProjectGraphSnapshotBasis{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return ProjectGraphSnapshotBasis{}, fmt.Errorf("decode project graph snapshot project: %w", err)
	}
	revisionValue, err := reader.readUint64("graph revision")
	if err != nil {
		return ProjectGraphSnapshotBasis{}, err
	}
	revision := typedmemory.NewGraphRevision(revisionValue)
	closureTag, err := reader.readString("closure kind")
	if err != nil {
		return ProjectGraphSnapshotBasis{}, err
	}
	closure, err := decodeProjectGraphClosure(&reader, closureTag)
	if err != nil {
		return ProjectGraphSnapshotBasis{}, err
	}
	if reader.remaining() != 0 {
		return ProjectGraphSnapshotBasis{}, fmt.Errorf("project graph snapshot basis has trailing bytes")
	}
	closure, err = normalizeProjectGraphClosure(revision, closure)
	if err != nil {
		return ProjectGraphSnapshotBasis{}, err
	}
	reencoded, err := encodeProjectGraphSnapshotBasis(
		project,
		revision,
		closure,
	)
	if err != nil {
		return ProjectGraphSnapshotBasis{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return ProjectGraphSnapshotBasis{}, fmt.Errorf("project graph snapshot basis is not canonical")
	}
	ref, err := deriveProjectGraphSnapshotBasisRef(canonical)
	if err != nil {
		return ProjectGraphSnapshotBasis{}, err
	}
	return ProjectGraphSnapshotBasis{
		ref:            ref,
		project:        project,
		revision:       revision,
		closure:        closure,
		canonicalBytes: append([]byte(nil), canonical...),
	}, nil
}

func VerifyProjectGraphSnapshotBasis(
	expected ProjectGraphSnapshotBasisRef,
	canonical []byte,
) (ProjectGraphSnapshotBasis, error) {
	parsed, err := ParseProjectGraphSnapshotBasisRef(expected.String())
	if err != nil || parsed != expected {
		return ProjectGraphSnapshotBasis{}, fmt.Errorf("expected project graph snapshot basis reference is invalid")
	}
	decoded, err := DecodeProjectGraphSnapshotBasis(canonical)
	if err != nil {
		return ProjectGraphSnapshotBasis{}, err
	}
	if decoded.ref != expected {
		return ProjectGraphSnapshotBasis{}, fmt.Errorf("project graph snapshot basis reference mismatch")
	}
	return decoded, nil
}

func (basis ProjectGraphSnapshotBasis) Ref() ProjectGraphSnapshotBasisRef { return basis.ref }

func (basis ProjectGraphSnapshotBasis) Project() projectidentity.ProjectID { return basis.project }

func (basis ProjectGraphSnapshotBasis) GraphRevision() typedmemory.GraphRevision {
	return basis.revision
}

func (basis ProjectGraphSnapshotBasis) Closure() ProjectGraphClosure { return basis.closure }

func (basis ProjectGraphSnapshotBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonicalBytes...)
}

func (basis ProjectGraphSnapshotBasis) Verify() error {
	verified, err := VerifyProjectGraphSnapshotBasis(basis.ref, basis.canonicalBytes)
	if err != nil {
		return err
	}
	if verified.project != basis.project ||
		verified.revision != basis.revision ||
		!projectGraphClosuresEqual(verified.closure, basis.closure) {
		return fmt.Errorf("project graph snapshot basis stored state differs from canonical bytes")
	}
	return nil
}

func normalizeProjectGraphClosure(
	revision typedmemory.GraphRevision,
	closure ProjectGraphClosure,
) (ProjectGraphClosure, error) {
	switch value := closure.(type) {
	case EmptyProjectGraphClosure:
		if revision.Value() != 0 {
			return nil, fmt.Errorf("non-zero graph revision requires committed closure")
		}
		return EmptyProjectGraphClosure{}, nil
	case CommittedProjectGraphClosure:
		if revision.Value() == 0 {
			return nil, fmt.Errorf("zero graph revision requires empty closure")
		}
		return NewCommittedProjectGraphClosure(CommittedProjectGraphClosureInput{
			Event:                 value.event,
			Commit:                value.commit,
			MaterializationDigest: value.materializationDigest,
		})
	default:
		return nil, fmt.Errorf("project graph closure is required")
	}
}

func encodeProjectGraphSnapshotBasis(
	project projectidentity.ProjectID,
	revision typedmemory.GraphRevision,
	closure ProjectGraphClosure,
) ([]byte, error) {
	writer := graphSnapshotWriter{}
	writer.addString(projectGraphSnapshotBasisDomain)
	writer.addString(project.String())
	writer.addUint64(revision.Value())
	switch value := closure.(type) {
	case EmptyProjectGraphClosure:
		writer.addString("empty")
	case CommittedProjectGraphClosure:
		writer.addString("committed")
		writer.addString(value.event.String())
		writer.addString(value.commit.String())
		writer.addString(value.materializationSchema)
		writer.addString(value.materializationDigest.String())
	default:
		return nil, fmt.Errorf("project graph closure is required")
	}
	result := writer.bytes()
	if len(result) > maximumProjectGraphSnapshotBasisBytes {
		return nil, fmt.Errorf(
			"project graph snapshot basis exceeds %d bytes",
			maximumProjectGraphSnapshotBasisBytes,
		)
	}
	return result, nil
}

func decodeProjectGraphClosure(
	reader *graphSnapshotReader,
	tag string,
) (ProjectGraphClosure, error) {
	switch tag {
	case "empty":
		return EmptyProjectGraphClosure{}, nil
	case "committed":
		eventText, err := reader.readString("event reference")
		if err != nil {
			return nil, err
		}
		event, err := ParseGraphEventRef(eventText)
		if err != nil {
			return nil, err
		}
		commitText, err := reader.readString("commit reference")
		if err != nil {
			return nil, err
		}
		commit, err := ParseGraphCommitRef(commitText)
		if err != nil {
			return nil, err
		}
		schema, err := reader.readString("materialization schema")
		if err != nil {
			return nil, err
		}
		if schema != ProjectGraphMaterializationSchemaV1 {
			return nil, fmt.Errorf("project graph materialization schema is unsupported")
		}
		digestText, err := reader.readString("materialization digest")
		if err != nil {
			return nil, err
		}
		digest, err := typedmemory.NewSHA256Digest(digestText)
		if err != nil {
			return nil, fmt.Errorf("decode project graph materialization digest: %w", err)
		}
		return NewCommittedProjectGraphClosure(CommittedProjectGraphClosureInput{
			Event:                 event,
			Commit:                commit,
			MaterializationDigest: digest,
		})
	default:
		return nil, fmt.Errorf("project graph closure kind %q is unsupported", tag)
	}
}

func deriveProjectGraphSnapshotBasisRef(
	canonical []byte,
) (ProjectGraphSnapshotBasisRef, error) {
	sum := sha256.Sum256(canonical)
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
	if err != nil {
		return ProjectGraphSnapshotBasisRef{}, err
	}
	return ProjectGraphSnapshotBasisRef{digest: digest}, nil
}

func projectGraphClosuresEqual(left ProjectGraphClosure, right ProjectGraphClosure) bool {
	switch leftValue := left.(type) {
	case EmptyProjectGraphClosure:
		_, ok := right.(EmptyProjectGraphClosure)
		return ok
	case CommittedProjectGraphClosure:
		rightValue, ok := right.(CommittedProjectGraphClosure)
		return ok && leftValue == rightValue
	default:
		return false
	}
}

type graphSnapshotWriter struct{ buffer bytes.Buffer }

func (writer *graphSnapshotWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *graphSnapshotWriter) addBytes(value []byte) {
	writer.addUint64(uint64(len(value)))
	writer.buffer.Write(value)
}

func (writer *graphSnapshotWriter) addUint64(value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.buffer.Write(encoded[:])
}

func (writer graphSnapshotWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type graphSnapshotReader struct {
	value  []byte
	offset int
}

func (reader *graphSnapshotReader) readUint64(label string) (uint64, error) {
	if len(reader.value)-reader.offset < 8 {
		return 0, fmt.Errorf("project graph snapshot %s is truncated", label)
	}
	value := binary.BigEndian.Uint64(reader.value[reader.offset : reader.offset+8])
	reader.offset += 8
	return value, nil
}

func (reader *graphSnapshotReader) readString(label string) (string, error) {
	length, err := reader.readUint64(label + " length")
	if err != nil {
		return "", err
	}
	lengthValue, exact := sliceIndexFromUint64(length)
	if !exact {
		return "", fmt.Errorf(
			"project graph snapshot %s length does not fit this runtime",
			label,
		)
	}
	if lengthValue > maximumProjectGraphCoordinateBytes {
		return "", fmt.Errorf("project graph snapshot %s exceeds %d bytes", label, maximumProjectGraphCoordinateBytes)
	}
	remaining := len(reader.value) - reader.offset
	if lengthValue > remaining {
		return "", fmt.Errorf("project graph snapshot %s is truncated", label)
	}
	end := reader.offset + lengthValue
	value := reader.value[reader.offset:end]
	reader.offset = end
	if !utf8.Valid(value) {
		return "", fmt.Errorf("project graph snapshot %s contains invalid UTF-8", label)
	}
	return string(value), nil
}

func (reader graphSnapshotReader) remaining() int { return len(reader.value) - reader.offset }
