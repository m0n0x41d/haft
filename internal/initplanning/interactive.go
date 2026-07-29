package initplanning

import (
	"fmt"
	"sort"
)

type SelectionPosture string

const (
	SelectionNotSelected SelectionPosture = "not_selected"
	SelectionSelected    SelectionPosture = "selected"
)

type InteractiveSessionInput struct {
	ProjectRoot string
	ProjectID   string
	Choices     []WeakHostSelection
	Discoveries []DiscoveryObservation
}

type interactiveChoice struct {
	selection HostSelection
	discovery DiscoveryObservation
}

type InteractiveSession struct {
	projectRoot string
	projectID   string
	catalog     AdapterCatalog
	choices     map[HostID]interactiveChoice
	selected    map[HostID]struct{}
	outcome     interactiveOutcomeKind
	confirmed   InitIntent
}

type interactiveOutcomeKind string

const (
	interactiveEditing   interactiveOutcomeKind = "editing"
	interactiveConfirmed interactiveOutcomeKind = "confirmed"
	interactiveCancelled interactiveOutcomeKind = "cancelled"
	interactiveEOF       interactiveOutcomeKind = "eof"
)

func NewInteractiveSession(
	input InteractiveSessionInput,
	catalog AdapterCatalog,
) (InteractiveSession, error) {
	base, err := ParseInitIntent(WeakInitIntent{
		InvocationPolicy: string(InvocationInteractive),
		ProjectRoot:      input.ProjectRoot,
		ProjectID:        input.ProjectID,
	})
	if err != nil {
		return InteractiveSession{}, err
	}
	if len(input.Choices) == 0 {
		return InteractiveSession{}, fmt.Errorf("interactive session needs at least one host choice")
	}
	choices := make(map[HostID]interactiveChoice, len(input.Choices))
	for _, weak := range input.Choices {
		selection, err := parseHostSelection(weak)
		if err != nil {
			return InteractiveSession{}, err
		}
		if err := catalog.validate(selection); err != nil {
			return InteractiveSession{}, err
		}
		if _, duplicate := choices[selection.host]; duplicate {
			return InteractiveSession{}, fmt.Errorf("interactive choice repeats host %s", selection.host)
		}
		choices[selection.host] = interactiveChoice{selection: selection}
	}
	for _, observation := range input.Discoveries {
		choice, ok := choices[observation.host]
		if !ok {
			return InteractiveSession{}, fmt.Errorf(
				"discovery for host %s has no interactive choice",
				observation.host,
			)
		}
		if choice.discovery.host != "" {
			return InteractiveSession{}, fmt.Errorf(
				"interactive discovery repeats host %s",
				observation.host,
			)
		}
		choice.discovery = observation
		choices[observation.host] = choice
	}
	return InteractiveSession{
		projectRoot: base.projectRoot,
		projectID:   base.projectID.String(),
		catalog:     catalog,
		choices:     cloneInteractiveChoices(choices),
		selected:    make(map[HostID]struct{}),
		outcome:     interactiveEditing,
	}, nil
}

type InteractiveEvent interface {
	interactiveEvent()
}

type ToggleHostEvent struct {
	host HostID
}

func NewToggleHostEvent(host HostID) (ToggleHostEvent, error) {
	if _, known := knownHosts[host]; !known {
		return ToggleHostEvent{}, fmt.Errorf("toggle host is not canonical")
	}
	return ToggleHostEvent{host: host}, nil
}

func (ToggleHostEvent) interactiveEvent() {}

type ConfirmSelectionEvent struct{}

func (ConfirmSelectionEvent) interactiveEvent() {}

type CancelSelectionEvent struct{}

func (CancelSelectionEvent) interactiveEvent() {}

type EndOfInputEvent struct{}

func (EndOfInputEvent) interactiveEvent() {}

func (session InteractiveSession) Reduce(
	event InteractiveEvent,
) (InteractiveSession, error) {
	if session.outcome != interactiveEditing {
		return InteractiveSession{}, fmt.Errorf("interactive initialization session is terminal")
	}
	switch typed := event.(type) {
	case ToggleHostEvent:
		return session.toggle(typed.host)
	case ConfirmSelectionEvent:
		return session.confirm()
	case CancelSelectionEvent:
		return session.terminate(interactiveCancelled), nil
	case EndOfInputEvent:
		return session.terminate(interactiveEOF), nil
	default:
		return InteractiveSession{}, fmt.Errorf("interactive initialization event is not closed")
	}
}

func (session InteractiveSession) toggle(
	host HostID,
) (InteractiveSession, error) {
	if _, available := session.choices[host]; !available {
		return InteractiveSession{}, fmt.Errorf("host %s is not an interactive choice", host)
	}
	next := session.clone()
	if _, selected := next.selected[host]; selected {
		delete(next.selected, host)
		return next, nil
	}
	next.selected[host] = struct{}{}
	return next, nil
}

func (session InteractiveSession) confirm() (InteractiveSession, error) {
	hostIDs := make([]HostID, 0, len(session.selected))
	for host := range session.selected {
		hostIDs = append(hostIDs, host)
	}
	sort.Slice(hostIDs, func(left int, right int) bool {
		return hostIDs[left] < hostIDs[right]
	})
	hosts := make([]WeakHostSelection, 0, len(hostIDs))
	for _, host := range hostIDs {
		selection := session.choices[host].selection
		components := selection.components.Values()
		rawComponents := make([]string, len(components))
		for index, component := range components {
			rawComponents[index] = string(component)
		}
		hosts = append(hosts, WeakHostSelection{
			Host:       string(selection.host),
			Scope:      string(selection.scope),
			Components: rawComponents,
		})
	}
	intent, err := ParseInitIntent(WeakInitIntent{
		InvocationPolicy: string(InvocationInteractive),
		ProjectRoot:      session.projectRoot,
		ProjectID:        session.projectID,
		Hosts:            hosts,
	})
	if err != nil {
		return InteractiveSession{}, err
	}
	next := session.clone()
	next.outcome = interactiveConfirmed
	next.confirmed = intent
	return next, nil
}

func (session InteractiveSession) terminate(
	outcome interactiveOutcomeKind,
) InteractiveSession {
	next := session.clone()
	next.outcome = outcome
	next.confirmed = InitIntent{}
	return next
}

func (session InteractiveSession) clone() InteractiveSession {
	selected := make(map[HostID]struct{}, len(session.selected))
	for host := range session.selected {
		selected[host] = struct{}{}
	}
	return InteractiveSession{
		projectRoot: session.projectRoot,
		projectID:   session.projectID,
		catalog:     session.catalog,
		choices:     cloneInteractiveChoices(session.choices),
		selected:    selected,
		outcome:     session.outcome,
		confirmed:   cloneInitIntent(session.confirmed),
	}
}

func cloneInteractiveChoices(
	source map[HostID]interactiveChoice,
) map[HostID]interactiveChoice {
	result := make(map[HostID]interactiveChoice, len(source))
	for host, choice := range source {
		result[host] = interactiveChoice{
			selection: HostSelection{
				host:       choice.selection.host,
				scope:      choice.selection.scope,
				components: choice.selection.Components(),
			},
			discovery: choice.discovery,
		}
	}
	return result
}

type InteractiveOutcome interface {
	interactiveOutcome()
}

type InteractiveOptionView struct {
	Host             HostID
	Scope            InstallScope
	Components       []Component
	Selection        SelectionPosture
	DiscoveryPosture DiscoveryPosture
	DiscoveryBasis   string
}

type InteractiveEditingOutcome struct {
	Options []InteractiveOptionView
}

func (InteractiveEditingOutcome) interactiveOutcome() {}

type InteractiveConfirmedOutcome struct {
	Intent InitIntent
}

func (InteractiveConfirmedOutcome) interactiveOutcome() {}

type InteractiveCancelledOutcome struct{}

func (InteractiveCancelledOutcome) interactiveOutcome() {}

type InteractiveEOFOutcome struct{}

func (InteractiveEOFOutcome) interactiveOutcome() {}

func (session InteractiveSession) Outcome() InteractiveOutcome {
	switch session.outcome {
	case interactiveConfirmed:
		return InteractiveConfirmedOutcome{Intent: cloneInitIntent(session.confirmed)}
	case interactiveCancelled:
		return InteractiveCancelledOutcome{}
	case interactiveEOF:
		return InteractiveEOFOutcome{}
	default:
		return InteractiveEditingOutcome{Options: session.optionViews()}
	}
}

func (session InteractiveSession) optionViews() []InteractiveOptionView {
	hosts := make([]HostID, 0, len(session.choices))
	for host := range session.choices {
		hosts = append(hosts, host)
	}
	sort.Slice(hosts, func(left int, right int) bool {
		return hosts[left] < hosts[right]
	})
	result := make([]InteractiveOptionView, len(hosts))
	for index, host := range hosts {
		choice := session.choices[host]
		selection := SelectionNotSelected
		if _, selected := session.selected[host]; selected {
			selection = SelectionSelected
		}
		discoveryPosture := DiscoveryNotDetected
		discoveryBasis := "not_observed"
		if choice.discovery.host != "" {
			discoveryPosture = choice.discovery.posture
			discoveryBasis = choice.discovery.basis
		}
		result[index] = InteractiveOptionView{
			Host:             host,
			Scope:            choice.selection.scope,
			Components:       choice.selection.components.Values(),
			Selection:        selection,
			DiscoveryPosture: discoveryPosture,
			DiscoveryBasis:   discoveryBasis,
		}
	}
	return result
}
