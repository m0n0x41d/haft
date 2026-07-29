package projecttypeenvselectionauthority

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	genesisActionValue    = "project-typeenv.head.genesis"
	transitionActionValue = "project-typeenv.head.transition"
)

type ProjectTypeEnvHeadSelectionAction uint8

const (
	ProjectTypeEnvHeadSelectionGenesis ProjectTypeEnvHeadSelectionAction = iota + 1
	ProjectTypeEnvHeadSelectionTransition
)

func (action ProjectTypeEnvHeadSelectionAction) String() string {
	switch action {
	case ProjectTypeEnvHeadSelectionGenesis:
		return "genesis"
	case ProjectTypeEnvHeadSelectionTransition:
		return "transition"
	default:
		return ""
	}
}

func (action ProjectTypeEnvHeadSelectionAction) AuthorityActionKind() (
	authority.ActionKind,
	error,
) {
	value := ""
	switch action {
	case ProjectTypeEnvHeadSelectionGenesis:
		value = genesisActionValue
	case ProjectTypeEnvHeadSelectionTransition:
		value = transitionActionValue
	default:
		return authority.ActionKind{}, fmt.Errorf("head-selection action is invalid")
	}
	kind, err := authority.NewActionKind(value)
	if err != nil {
		return authority.ActionKind{}, err
	}
	return kind, nil
}

// ProjectTypeEnvHeadUpdate is the closed intended-effect projection. It is
// derived from an already verified request and cannot represent Genesis with a
// fabricated prior revision or Transition without an exact prior head.
type ProjectTypeEnvHeadUpdate struct {
	action            ProjectTypeEnvHeadSelectionAction
	head              projecttypeenvselection.ProjectTypeEnvHeadRef
	priorComposite    typedmemory.TypeEnvRef
	priorRevision     projecttypeenvselection.HeadRevision
	selectedComposite typedmemory.TypeEnvRef
	successorRevision projecttypeenvselection.HeadRevision
}

func deriveHeadUpdate(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
) (ProjectTypeEnvHeadUpdate, error) {
	if err := request.Verify(); err != nil {
		return ProjectTypeEnvHeadUpdate{}, err
	}
	head, err := request.Head()
	if err != nil {
		return ProjectTypeEnvHeadUpdate{}, err
	}
	update := ProjectTypeEnvHeadUpdate{
		head:              head,
		selectedComposite: request.Target().VerifiedComposite(),
	}
	switch predecessor := request.Predecessor().(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		update.action = ProjectTypeEnvHeadSelectionGenesis
		revision, revisionErr := projecttypeenvselection.NewHeadRevision(1)
		if revisionErr != nil {
			return ProjectTypeEnvHeadUpdate{}, revisionErr
		}
		update.successorRevision = revision
	case projecttypeenvselection.TransitionStagePredecessor:
		update.action = ProjectTypeEnvHeadSelectionTransition
		update.priorComposite = predecessor.SelectedComposite()
		update.priorRevision = predecessor.HeadRevision()
		value := predecessor.HeadRevision().Value()
		if value == ^uint64(0) {
			return ProjectTypeEnvHeadUpdate{}, projecttypeenvselection.ErrProjectTypeEnvHeadRevisionOverflow
		}
		revision, revisionErr := projecttypeenvselection.NewHeadRevision(value + 1)
		if revisionErr != nil {
			return ProjectTypeEnvHeadUpdate{}, revisionErr
		}
		update.successorRevision = revision
	default:
		return ProjectTypeEnvHeadUpdate{}, fmt.Errorf("head-selection predecessor posture is invalid")
	}
	return update, nil
}

func (update ProjectTypeEnvHeadUpdate) Action() ProjectTypeEnvHeadSelectionAction {
	return update.action
}

func (update ProjectTypeEnvHeadUpdate) Head() projecttypeenvselection.ProjectTypeEnvHeadRef {
	return update.head
}

func (update ProjectTypeEnvHeadUpdate) PriorComposite() (typedmemory.TypeEnvRef, bool) {
	return update.priorComposite, update.action == ProjectTypeEnvHeadSelectionTransition
}

func (update ProjectTypeEnvHeadUpdate) PriorRevision() (
	projecttypeenvselection.HeadRevision,
	bool,
) {
	return update.priorRevision, update.action == ProjectTypeEnvHeadSelectionTransition
}

func (update ProjectTypeEnvHeadUpdate) SelectedComposite() typedmemory.TypeEnvRef {
	return update.selectedComposite
}

func (update ProjectTypeEnvHeadUpdate) SuccessorRevision() projecttypeenvselection.HeadRevision {
	return update.successorRevision
}
