package projecttypeenvselection

import (
	"bytes"
	"errors"
	"fmt"
	"math"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	projectTypeEnvHeadStateDomain = "haft.project-typeenv.head-state.v1"
	maximumHeadStateBytes         = 64 << 10
)

var ErrProjectTypeEnvHeadRevisionOverflow = errors.New(
	"project TypeEnv head revision overflow",
)

// ProjectTypeEnvHeadState is an immutable exact coordinate of one head
// revision. Its canonical bytes do not prove that this is the current stored
// head. A storage-owned witness and CAS reread are required at the effect
// boundary. This type is not the later durable receipt and intentionally
// contains no GraphRevision, authority, SpeechAct, or Work coordinate.
type ProjectTypeEnvHeadState struct {
	project           projectidentity.ProjectID
	head              ProjectTypeEnvHeadRef
	selectedComposite typedmemory.TypeEnvRef
	revision          HeadRevision
	canonicalBytes    []byte
}

type ProjectTypeEnvHeadStateInput struct {
	Project           projectidentity.ProjectID
	SelectedComposite typedmemory.TypeEnvRef
	Revision          HeadRevision
}

func SealProjectTypeEnvHeadState(
	input ProjectTypeEnvHeadStateInput,
) (ProjectTypeEnvHeadState, error) {
	head, err := ProjectTypeEnvHeadRefForProject(input.Project)
	if err != nil {
		return ProjectTypeEnvHeadState{}, err
	}
	state, err := normalizeProjectTypeEnvHeadState(projectTypeEnvHeadStateValue{
		project:           input.Project,
		head:              head,
		selectedComposite: input.SelectedComposite,
		revision:          input.Revision,
	})
	if err != nil {
		return ProjectTypeEnvHeadState{}, err
	}
	return DecodeProjectTypeEnvHeadState(encodeProjectTypeEnvHeadState(state))
}

func DecodeProjectTypeEnvHeadState(canonical []byte) (ProjectTypeEnvHeadState, error) {
	if len(canonical) == 0 {
		return ProjectTypeEnvHeadState{}, fmt.Errorf("project TypeEnv head state is empty")
	}
	if len(canonical) > maximumHeadStateBytes {
		return ProjectTypeEnvHeadState{}, fmt.Errorf(
			"project TypeEnv head state exceeds %d bytes",
			maximumHeadStateBytes,
		)
	}
	reader := stageReader{value: canonical}
	domain, err := reader.readString("head-state domain")
	if err != nil {
		return ProjectTypeEnvHeadState{}, err
	}
	if domain != projectTypeEnvHeadStateDomain {
		return ProjectTypeEnvHeadState{}, fmt.Errorf("project TypeEnv head-state domain is invalid")
	}
	state, err := decodeProjectTypeEnvHeadState(&reader)
	if err != nil {
		return ProjectTypeEnvHeadState{}, err
	}
	if reader.remaining() != 0 {
		return ProjectTypeEnvHeadState{}, fmt.Errorf("project TypeEnv head state has trailing bytes")
	}
	normalized, err := normalizeProjectTypeEnvHeadState(state)
	if err != nil {
		return ProjectTypeEnvHeadState{}, err
	}
	reencoded := encodeProjectTypeEnvHeadState(normalized)
	if !bytes.Equal(reencoded, canonical) {
		return ProjectTypeEnvHeadState{}, fmt.Errorf("project TypeEnv head state is not canonical")
	}
	return ProjectTypeEnvHeadState{
		project:           normalized.project,
		head:              normalized.head,
		selectedComposite: normalized.selectedComposite,
		revision:          normalized.revision,
		canonicalBytes:    append([]byte(nil), canonical...),
	}, nil
}

func (state ProjectTypeEnvHeadState) Project() projectidentity.ProjectID {
	return state.project
}

func (state ProjectTypeEnvHeadState) Ref() ProjectTypeEnvHeadRef { return state.head }

func (state ProjectTypeEnvHeadState) SelectedComposite() typedmemory.TypeEnvRef {
	return state.selectedComposite
}

func (state ProjectTypeEnvHeadState) Revision() HeadRevision { return state.revision }

func (state ProjectTypeEnvHeadState) CanonicalBytes() []byte {
	return append([]byte(nil), state.canonicalBytes...)
}

func (state ProjectTypeEnvHeadState) Verify() error {
	decoded, err := DecodeProjectTypeEnvHeadState(state.canonicalBytes)
	if err != nil {
		return err
	}
	if decoded.project != state.project ||
		decoded.head != state.head ||
		decoded.selectedComposite != state.selectedComposite ||
		decoded.revision != state.revision {
		return fmt.Errorf("project TypeEnv head stored state differs from canonical bytes")
	}
	return nil
}

func (state ProjectTypeEnvHeadState) ExactPriorHead() (TransitionStagePredecessor, error) {
	if err := state.Verify(); err != nil {
		return TransitionStagePredecessor{}, err
	}
	return NewTransitionStagePredecessor(TransitionStagePredecessorInput{
		Project:           state.project,
		Head:              state.head,
		HeadRevision:      state.revision,
		SelectedComposite: state.selectedComposite,
	})
}

// DeriveGenesisProjectTypeEnvHeadSuccessorCandidate computes only an immutable
// structural successor candidate. It does not prove current head absence and
// performs no CAS, graph write, authority use, or head selection.
func DeriveGenesisProjectTypeEnvHeadSuccessorCandidate(
	request ProjectTypeEnvHeadSelectionRequest,
	stage ProjectTypeEnvStage,
) (ProjectTypeEnvHeadState, error) {
	if err := VerifyGenesisProjectTypeEnvHeadSelectionRequestStructure(
		request,
		stage,
	); err != nil {
		return ProjectTypeEnvHeadState{}, err
	}
	return deriveProjectTypeEnvHeadSuccessorCandidate(request)
}

// DeriveTransitionProjectTypeEnvHeadSuccessorCandidate computes only an
// immutable structural successor candidate. It does not prove that prior is
// the current stored head and performs no CAS, graph write, authority use, or
// head selection.
func DeriveTransitionProjectTypeEnvHeadSuccessorCandidate(
	request ProjectTypeEnvHeadSelectionRequest,
	prior ProjectTypeEnvHeadState,
	stage ProjectTypeEnvStage,
) (ProjectTypeEnvHeadState, error) {
	if err := VerifyTransitionProjectTypeEnvHeadSelectionRequestStructure(
		request,
		prior,
		stage,
	); err != nil {
		return ProjectTypeEnvHeadState{}, err
	}
	return deriveProjectTypeEnvHeadSuccessorCandidate(request)
}

func deriveProjectTypeEnvHeadSuccessorCandidate(
	request ProjectTypeEnvHeadSelectionRequest,
) (ProjectTypeEnvHeadState, error) {
	if err := request.Verify(); err != nil {
		return ProjectTypeEnvHeadState{}, err
	}
	revisionValue, err := successorHeadRevisionValue(request.predecessor)
	if err != nil {
		return ProjectTypeEnvHeadState{}, err
	}
	revision, err := NewHeadRevision(revisionValue)
	if err != nil {
		return ProjectTypeEnvHeadState{}, err
	}
	return SealProjectTypeEnvHeadState(ProjectTypeEnvHeadStateInput{
		Project:           request.project,
		SelectedComposite: request.target.verifiedComposite,
		Revision:          revision,
	})
}

func successorHeadRevisionValue(
	predecessor ProjectTypeEnvHeadSelectionPredecessor,
) (uint64, error) {
	switch value := predecessor.(type) {
	case GenesisStagePredecessor:
		return 1, nil
	case TransitionStagePredecessor:
		if value.HeadRevision().Value() == math.MaxUint64 {
			return 0, ErrProjectTypeEnvHeadRevisionOverflow
		}
		return value.HeadRevision().Value() + 1, nil
	default:
		return 0, fmt.Errorf("head-selection predecessor posture is required")
	}
}

type projectTypeEnvHeadStateValue struct {
	project           projectidentity.ProjectID
	head              ProjectTypeEnvHeadRef
	selectedComposite typedmemory.TypeEnvRef
	revision          HeadRevision
}

func normalizeProjectTypeEnvHeadState(
	state projectTypeEnvHeadStateValue,
) (projectTypeEnvHeadStateValue, error) {
	project, err := projectidentity.ParseProjectID(state.project.String())
	if err != nil || project != state.project {
		return projectTypeEnvHeadStateValue{}, fmt.Errorf("project TypeEnv head-state project is required")
	}
	expectedHead, err := ProjectTypeEnvHeadRefForProject(project)
	if err != nil {
		return projectTypeEnvHeadStateValue{}, err
	}
	head := state.head
	parsedHead, err := ParseProjectTypeEnvHeadRef(head.String())
	if err != nil || parsedHead != head || head != expectedHead {
		return projectTypeEnvHeadStateValue{}, fmt.Errorf("project TypeEnv head-state head mismatch")
	}
	composite, err := normalizeTypeEnvRef(
		"project TypeEnv head-state selected composite",
		state.selectedComposite,
	)
	if err != nil {
		return projectTypeEnvHeadStateValue{}, err
	}
	revision, err := NewHeadRevision(state.revision.Value())
	if err != nil || revision != state.revision {
		return projectTypeEnvHeadStateValue{}, fmt.Errorf("project TypeEnv head revision is required")
	}
	return projectTypeEnvHeadStateValue{
		project:           project,
		head:              head,
		selectedComposite: composite,
		revision:          revision,
	}, nil
}

func encodeProjectTypeEnvHeadState(state projectTypeEnvHeadStateValue) []byte {
	writer := stageWriter{}
	writer.addString(projectTypeEnvHeadStateDomain)
	writer.addString(state.project.String())
	writer.addString(state.head.String())
	writer.addString(state.selectedComposite.String())
	writer.addUint64(state.revision.Value())
	return writer.bytes()
}

func decodeProjectTypeEnvHeadState(
	reader *stageReader,
) (projectTypeEnvHeadStateValue, error) {
	projectText, err := reader.readString("head-state project")
	if err != nil {
		return projectTypeEnvHeadStateValue{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return projectTypeEnvHeadStateValue{}, err
	}
	headText, err := reader.readString("head-state head")
	if err != nil {
		return projectTypeEnvHeadStateValue{}, err
	}
	head, err := ParseProjectTypeEnvHeadRef(headText)
	if err != nil {
		return projectTypeEnvHeadStateValue{}, err
	}
	compositeText, err := reader.readString("head-state selected composite")
	if err != nil {
		return projectTypeEnvHeadStateValue{}, err
	}
	composite, err := typedmemory.ParseTypeEnvRef(compositeText)
	if err != nil {
		return projectTypeEnvHeadStateValue{}, err
	}
	revisionValue, err := reader.readUint64("head-state revision")
	if err != nil {
		return projectTypeEnvHeadStateValue{}, err
	}
	revision, err := NewHeadRevision(revisionValue)
	if err != nil {
		return projectTypeEnvHeadStateValue{}, err
	}
	return projectTypeEnvHeadStateValue{
		project:           project,
		head:              head,
		selectedComposite: composite,
		revision:          revision,
	}, nil
}
