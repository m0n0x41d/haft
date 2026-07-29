package projecttypeenvstagerevalidation

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
)

// CurrentProjectTypeEnvHeadObservation is a closed, non-authorizing
// observation algebra. It does not prove that a database read occurred and
// cannot mint the separate CAS/head-absence capabilities owned by the future
// effect shell.
type CurrentProjectTypeEnvHeadObservation interface {
	Project() projectidentity.ProjectID
	currentProjectTypeEnvHeadObservationVariant()
}

// ObservedNoProjectTypeEnvHead carries an exact project/head-slot coordinate
// whose absence was reported by a caller. The future transaction adapter must
// construct it only after a same-transaction read; this pure value itself is
// not storage evidence.
type ObservedNoProjectTypeEnvHead struct {
	project projectidentity.ProjectID
	head    projecttypeenvselection.ProjectTypeEnvHeadRef
}

func NewObservedNoProjectTypeEnvHead(
	project projectidentity.ProjectID,
) (ObservedNoProjectTypeEnvHead, error) {
	canonical, err := projectidentity.ParseProjectID(project.String())
	if err != nil || canonical != project {
		return ObservedNoProjectTypeEnvHead{}, fmt.Errorf(
			"observed absent project TypeEnv head requires an exact project",
		)
	}
	head, err := projecttypeenvselection.ProjectTypeEnvHeadRefForProject(canonical)
	if err != nil {
		return ObservedNoProjectTypeEnvHead{}, err
	}
	return ObservedNoProjectTypeEnvHead{
		project: canonical,
		head:    head,
	}, nil
}

func (observation ObservedNoProjectTypeEnvHead) Project() projectidentity.ProjectID {
	return observation.project
}

func (observation ObservedNoProjectTypeEnvHead) Head() projecttypeenvselection.ProjectTypeEnvHeadRef {
	return observation.head
}

func (ObservedNoProjectTypeEnvHead) currentProjectTypeEnvHeadObservationVariant() {}

func (observation ObservedNoProjectTypeEnvHead) verify() error {
	canonical, err := NewObservedNoProjectTypeEnvHead(observation.project)
	if err != nil {
		return err
	}
	if canonical.head != observation.head {
		return fmt.Errorf("observed absent project TypeEnv head coordinate is inconsistent")
	}
	return nil
}

// ObservedProjectTypeEnvHead carries one exact immutable head state. Its bytes
// remain data, not proof that this state is current in storage.
type ObservedProjectTypeEnvHead struct {
	state projecttypeenvselection.ProjectTypeEnvHeadState
}

func NewObservedProjectTypeEnvHead(
	state projecttypeenvselection.ProjectTypeEnvHeadState,
) (ObservedProjectTypeEnvHead, error) {
	if err := state.Verify(); err != nil {
		return ObservedProjectTypeEnvHead{}, fmt.Errorf(
			"observed project TypeEnv head state: %w",
			err,
		)
	}
	return ObservedProjectTypeEnvHead{state: state}, nil
}

func (observation ObservedProjectTypeEnvHead) Project() projectidentity.ProjectID {
	return observation.state.Project()
}

func (observation ObservedProjectTypeEnvHead) State() projecttypeenvselection.ProjectTypeEnvHeadState {
	return observation.state
}

func (ObservedProjectTypeEnvHead) currentProjectTypeEnvHeadObservationVariant() {}

func (observation ObservedProjectTypeEnvHead) verify() error {
	_, err := NewObservedProjectTypeEnvHead(observation.state)
	return err
}

func verifyHeadObservation(observation CurrentProjectTypeEnvHeadObservation) error {
	switch value := observation.(type) {
	case ObservedNoProjectTypeEnvHead:
		return value.verify()
	case ObservedProjectTypeEnvHead:
		return value.verify()
	default:
		return fmt.Errorf("current project TypeEnv head observation is required")
	}
}
