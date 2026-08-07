package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

type initSelectionTUIModel struct {
	session initplanning.InteractiveSession
	cursor  int
	failure error
}

func newInitSelectionTUIModel(
	session initplanning.InteractiveSession,
) (initSelectionTUIModel, error) {
	editing, ok := session.Outcome().(initplanning.InteractiveEditingOutcome)
	if !ok {
		return initSelectionTUIModel{}, fmt.Errorf(
			"initialization selection session is already terminal",
		)
	}
	if len(editing.Options) == 0 {
		return initSelectionTUIModel{}, fmt.Errorf(
			"initialization selection session has no host options",
		)
	}
	return initSelectionTUIModel{session: session}, nil
}

func (model initSelectionTUIModel) Init() tea.Cmd {
	return nil
}

func (model initSelectionTUIModel) Update(
	message tea.Msg,
) (tea.Model, tea.Cmd) {
	switch typed := message.(type) {
	case tea.KeyPressMsg:
		return model.updateKey(typed)
	default:
		return model, nil
	}
}

func (model initSelectionTUIModel) updateKey(
	message tea.KeyPressMsg,
) (tea.Model, tea.Cmd) {
	key := message.Key()
	switch {
	case key.Code == tea.KeyUp || message.String() == "k":
		return model.moveCursor(-1), nil
	case key.Code == tea.KeyDown || message.String() == "j":
		return model.moveCursor(1), nil
	case key.Code == tea.KeySpace:
		return model.toggleCurrent()
	case key.Code == tea.KeyEnter:
		return model.reduceAndQuit(
			initplanning.ConfirmSelectionEvent{},
		)
	case message.String() == "ctrl+d":
		return model.reduceAndQuit(
			initplanning.EndOfInputEvent{},
		)
	case key.Code == tea.KeyEscape,
		message.String() == "q",
		message.String() == "ctrl+c":
		return model.reduceAndQuit(
			initplanning.CancelSelectionEvent{},
		)
	default:
		return model, nil
	}
}

func (model initSelectionTUIModel) moveCursor(
	delta int,
) initSelectionTUIModel {
	options, err := model.editingOptions()
	if err != nil {
		model.failure = err
		return model
	}
	count := len(options)
	model.cursor = (model.cursor + delta + count) % count
	return model
}

func (model initSelectionTUIModel) toggleCurrent() (
	tea.Model,
	tea.Cmd,
) {
	options, err := model.editingOptions()
	if err != nil {
		return model.withFailure(err), tea.Quit
	}
	host := options[model.cursor].Host
	event, err := initplanning.NewToggleHostEvent(host)
	if err != nil {
		return model.withFailure(err), tea.Quit
	}
	next, err := model.session.Reduce(event)
	if err != nil {
		return model.withFailure(err), tea.Quit
	}
	model.session = next
	return model, nil
}

func (model initSelectionTUIModel) reduceAndQuit(
	event initplanning.InteractiveEvent,
) (tea.Model, tea.Cmd) {
	next, err := model.session.Reduce(event)
	if err != nil {
		return model.withFailure(err), tea.Quit
	}
	model.session = next
	return model, tea.Quit
}

func (model initSelectionTUIModel) withFailure(
	err error,
) initSelectionTUIModel {
	model.failure = err
	return model
}

func (model initSelectionTUIModel) editingOptions() (
	[]initplanning.InteractiveOptionView,
	error,
) {
	editing, ok := model.session.Outcome().(initplanning.InteractiveEditingOutcome)
	if !ok {
		return nil, fmt.Errorf(
			"initialization selection session is terminal",
		)
	}
	if len(editing.Options) == 0 ||
		model.cursor < 0 ||
		model.cursor >= len(editing.Options) {
		return nil, fmt.Errorf(
			"initialization selection cursor is invalid",
		)
	}
	return editing.Options, nil
}

func (model initSelectionTUIModel) View() tea.View {
	outcome := model.session.Outcome()
	_, editing := outcome.(initplanning.InteractiveEditingOutcome)
	if !editing {
		view := tea.NewView("")
		view.AltScreen = true
		return view
	}
	builder := strings.Builder{}
	builder.WriteString("Haft initialization\n\n")
	builder.WriteString(
		"Core project/ledger will initialize or migrate.\n" +
			"Select every host to configure; each row names its exact effect.\n" +
			"Stable hosts carry the v9 compatibility contract; experimental hosts remain opt-in.\n\n",
	)
	options, err := model.editingOptions()
	if err != nil {
		fmt.Fprintf(&builder, "Selection unavailable: %v\n", err)
	}
	for index, option := range options {
		builder.WriteString(
			renderInitSelectionOption(index, model.cursor, option),
		)
		builder.WriteByte('\n')
	}
	if model.failure != nil {
		fmt.Fprintf(&builder, "\nSelection error: %v\n", model.failure)
	}
	builder.WriteString(
		"\n↑/↓ move  Space toggle  Enter apply  Esc cancel\n",
	)
	view := tea.NewView(builder.String())
	view.AltScreen = true
	return view
}

func renderInitSelectionOption(
	index int,
	cursor int,
	option initplanning.InteractiveOptionView,
) string {
	pointer := " "
	if index == cursor {
		pointer = ">"
	}
	checkbox := "[ ]"
	if option.Selection == initplanning.SelectionSelected {
		checkbox = "[x]"
	}
	discovery := ""
	if option.DiscoveryPosture == initplanning.DiscoveryDetected {
		discovery = fmt.Sprintf(
			"  suggested: %s",
			option.DiscoveryBasis,
		)
	}
	return fmt.Sprintf(
		"%s %s %s — %s%s",
		pointer,
		checkbox,
		initSelectionHostLabel(option.Host),
		initSelectionEffectLabel(option),
		discovery,
	)
}

func initSelectionHostLabel(
	host initplanning.HostID,
) string {
	posture := "experimental"
	if host == initplanning.HostClaude ||
		host == initplanning.HostCodex {
		posture = "stable"
	}
	return fmt.Sprintf(
		"%s (%s)",
		publicInitHostLabel(host),
		posture,
	)
}

func initSelectionEffectLabel(
	option initplanning.InteractiveOptionView,
) string {
	switch option.Host {
	case initplanning.HostClaude:
		return "MCP + global skills + CLAUDE.md"
	case initplanning.HostCodex:
		return "MCP + global skills + AGENTS.md"
	}
	labels := make([]string, 0, len(option.Components))
	for _, component := range option.Components {
		labels = append(
			labels,
			publicInitComponentLabel(component),
		)
	}
	effect := strings.Join(labels, " + ")
	if option.Scope == initplanning.ScopeUser {
		return effect + " (user scope)"
	}
	return effect
}

func runInitSelectionTUI(
	session initplanning.InteractiveSession,
	input io.Reader,
	output io.Writer,
) (initplanning.InteractiveOutcome, error) {
	if input == nil || output == nil {
		return nil, fmt.Errorf(
			"initialization selection terminal streams are required",
		)
	}
	model, err := newInitSelectionTUIModel(session)
	if err != nil {
		return nil, err
	}
	program := tea.NewProgram(
		model,
		tea.WithInput(initSelectionProgramInput(input)),
		tea.WithOutput(output),
		tea.WithWindowSize(80, 24),
	)
	final, runErr := program.Run()
	finalModel, ok := final.(initSelectionTUIModel)
	if !ok {
		return nil, fmt.Errorf(
			"initialization selection returned an unexpected terminal model",
		)
	}
	if finalModel.failure != nil {
		return nil, finalModel.failure
	}
	if runErr != nil && !errors.Is(runErr, io.EOF) {
		return nil, fmt.Errorf(
			"run initialization selection TUI: %w",
			runErr,
		)
	}
	outcome := finalModel.session.Outcome()
	if _, editing := outcome.(initplanning.InteractiveEditingOutcome); !editing {
		return outcome, nil
	}
	eofSession, err := finalModel.session.Reduce(
		initplanning.EndOfInputEvent{},
	)
	if err != nil {
		return nil, err
	}
	return eofSession.Outcome(), nil
}

func initSelectionProgramInput(input io.Reader) io.Reader {
	if _, terminal := input.(*os.File); terminal {
		return input
	}
	return &initSelectionEOFReader{source: input}
}

type initSelectionEOFReader struct {
	source     io.Reader
	emittedEOF bool
}

func (reader *initSelectionEOFReader) Read(
	target []byte,
) (int, error) {
	count, err := reader.source.Read(target)
	if count > 0 {
		return count, nil
	}
	if !errors.Is(err, io.EOF) || reader.emittedEOF {
		return count, err
	}
	if len(target) == 0 {
		return 0, io.ErrShortBuffer
	}
	reader.emittedEOF = true
	target[0] = 0x04
	return 1, nil
}
