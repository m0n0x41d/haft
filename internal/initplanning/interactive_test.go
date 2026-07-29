package initplanning

import (
	"reflect"
	"strings"
	"testing"
)

func TestInteractiveReducerKeepsDiscoveryAsSuggestionUntilToggle(t *testing.T) {
	root := canonicalTempRoot(t)
	mcp := mustComponents(t, ComponentMCP)
	skillsMCP := mustComponents(t, ComponentMCP, ComponentSkills)
	codex := mustCapability(t, HostCodex, "codex.v1", ScopeProject, skillsMCP)
	zed := mustCapability(t, HostZed, "zed.v1", ScopeUser, mcp)
	catalog, err := NewAdapterCatalog([]AdapterCapability{zed, codex})
	if err != nil {
		t.Fatalf("NewAdapterCatalog: %v", err)
	}
	discovery, err := NewDiscoveryObservation(
		HostCodex,
		"binary_found_on_path",
		DiscoveryDetected,
	)
	if err != nil {
		t.Fatalf("NewDiscoveryObservation: %v", err)
	}
	session, err := NewInteractiveSession(InteractiveSessionInput{
		ProjectRoot: root,
		ProjectID:   "qnt_e3149c17",
		Choices: []WeakHostSelection{
			{
				Host:       string(HostZed),
				Scope:      string(ScopeUser),
				Components: []string{string(ComponentMCP)},
			},
			{
				Host:       string(HostCodex),
				Scope:      string(ScopeProject),
				Components: []string{string(ComponentSkills), string(ComponentMCP)},
			},
		},
		Discoveries: []DiscoveryObservation{discovery},
	}, catalog)
	if err != nil {
		t.Fatalf("NewInteractiveSession: %v", err)
	}
	editing, ok := session.Outcome().(InteractiveEditingOutcome)
	if !ok {
		t.Fatalf("initial outcome = %T, want editing", session.Outcome())
	}
	if editing.Options[0].Host != HostCodex ||
		editing.Options[0].DiscoveryPosture != DiscoveryDetected ||
		editing.Options[0].Selection != SelectionNotSelected {
		t.Fatalf("detected Codex option = %+v", editing.Options[0])
	}

	toggle, err := NewToggleHostEvent(HostCodex)
	if err != nil {
		t.Fatalf("NewToggleHostEvent: %v", err)
	}
	selected, err := session.Reduce(toggle)
	if err != nil {
		t.Fatalf("toggle Codex: %v", err)
	}
	confirmed, err := selected.Reduce(ConfirmSelectionEvent{})
	if err != nil {
		t.Fatalf("confirm selection: %v", err)
	}
	outcome, ok := confirmed.Outcome().(InteractiveConfirmedOutcome)
	if !ok {
		t.Fatalf("confirmed outcome = %T", confirmed.Outcome())
	}
	hosts := outcome.Intent.SelectedHosts().Values()
	if len(hosts) != 1 || hosts[0].Host() != HostCodex {
		t.Fatalf("confirmed hosts = %+v", hosts)
	}
	if outcome.Intent.InvocationPolicy() != InvocationInteractive {
		t.Fatalf("confirmed policy = %s", outcome.Intent.InvocationPolicy())
	}

	initialAgain := session.Outcome().(InteractiveEditingOutcome)
	if initialAgain.Options[0].Selection != SelectionNotSelected {
		t.Fatal("immutable reducer mutated the prior interactive state")
	}
}

func TestInteractiveConfirmWithNoToggleProducesExplicitCoreOnlyIntent(t *testing.T) {
	session := mustInteractiveSession(t)
	confirmed, err := session.Reduce(ConfirmSelectionEvent{})
	if err != nil {
		t.Fatalf("confirm core-only: %v", err)
	}
	outcome, ok := confirmed.Outcome().(InteractiveConfirmedOutcome)
	if !ok {
		t.Fatalf("outcome = %T, want confirmed", confirmed.Outcome())
	}
	if len(outcome.Intent.SelectedHosts().Values()) != 0 {
		t.Fatalf("core-only confirmation selected hosts: %v", outcome.Intent.SelectedHosts().Values())
	}
}

func TestInteractiveCancelAndEOFAreDistinctTerminalNoWriteOutcomes(t *testing.T) {
	cancelled, err := mustInteractiveSession(t).Reduce(CancelSelectionEvent{})
	if err != nil {
		t.Fatalf("cancel selection: %v", err)
	}
	if _, ok := cancelled.Outcome().(InteractiveCancelledOutcome); !ok {
		t.Fatalf("cancel outcome = %T", cancelled.Outcome())
	}
	eof, err := mustInteractiveSession(t).Reduce(EndOfInputEvent{})
	if err != nil {
		t.Fatalf("end input: %v", err)
	}
	if _, ok := eof.Outcome().(InteractiveEOFOutcome); !ok {
		t.Fatalf("EOF outcome = %T", eof.Outcome())
	}
	if _, err := cancelled.Reduce(ConfirmSelectionEvent{}); err == nil {
		t.Fatal("terminal cancelled session accepted another event")
	}
	if _, err := eof.Reduce(ConfirmSelectionEvent{}); err == nil {
		t.Fatal("terminal EOF session accepted another event")
	}
}

func TestInteractiveReducerToggleIsReversibleAndCanonical(t *testing.T) {
	session := mustInteractiveSession(t)
	toggle, err := NewToggleHostEvent(HostCodex)
	if err != nil {
		t.Fatalf("NewToggleHostEvent: %v", err)
	}
	selected, err := session.Reduce(toggle)
	if err != nil {
		t.Fatalf("select Codex: %v", err)
	}
	unselected, err := selected.Reduce(toggle)
	if err != nil {
		t.Fatalf("unselect Codex: %v", err)
	}
	initial := session.Outcome().(InteractiveEditingOutcome)
	final := unselected.Outcome().(InteractiveEditingOutcome)
	if !reflect.DeepEqual(final, initial) {
		t.Fatalf("toggle twice changed state\ninitial: %+v\nfinal:   %+v", initial, final)
	}
}

func TestInteractiveSessionRejectsUnsupportedAndUnpairedDiscovery(t *testing.T) {
	root := canonicalTempRoot(t)
	mcp := mustComponents(t, ComponentMCP)
	catalog := mustCatalog(t, HostZed, "zed.v1", ScopeUser, mcp)
	unsupported := InteractiveSessionInput{
		ProjectRoot: root,
		ProjectID:   "qnt_e3149c17",
		Choices: []WeakHostSelection{{
			Host:       string(HostZed),
			Scope:      string(ScopeProject),
			Components: []string{string(ComponentMCP)},
		}},
	}
	if _, err := NewInteractiveSession(unsupported, catalog); err == nil {
		t.Fatal("interactive session accepted an unsupported host scope")
	}
	discovery, err := NewDiscoveryObservation(
		HostCodex,
		"binary_found_on_path",
		DiscoveryDetected,
	)
	if err != nil {
		t.Fatalf("NewDiscoveryObservation: %v", err)
	}
	unpaired := InteractiveSessionInput{
		ProjectRoot: root,
		ProjectID:   "qnt_e3149c17",
		Choices: []WeakHostSelection{{
			Host:       string(HostZed),
			Scope:      string(ScopeUser),
			Components: []string{string(ComponentMCP)},
		}},
		Discoveries: []DiscoveryObservation{discovery},
	}
	_, err = NewInteractiveSession(unpaired, catalog)
	if err == nil || !strings.Contains(err.Error(), "no interactive choice") {
		t.Fatalf("unpaired discovery was accepted: %v", err)
	}
}

func mustInteractiveSession(t *testing.T) InteractiveSession {
	t.Helper()
	root := canonicalTempRoot(t)
	components := mustComponents(t, ComponentMCP, ComponentSkills)
	catalog := mustCatalog(t, HostCodex, "codex.v1", ScopeProject, components)
	session, err := NewInteractiveSession(InteractiveSessionInput{
		ProjectRoot: root,
		ProjectID:   "qnt_e3149c17",
		Choices: []WeakHostSelection{{
			Host:       string(HostCodex),
			Scope:      string(ScopeProject),
			Components: []string{string(ComponentMCP), string(ComponentSkills)},
		}},
	}, catalog)
	if err != nil {
		t.Fatalf("NewInteractiveSession: %v", err)
	}
	return session
}
