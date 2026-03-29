package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/m0n0x41d/quint-code/internal/agent"
	"github.com/m0n0x41d/quint-code/internal/session"
)

// RunFunc is the coordinator's Run function, injected to avoid import cycles.
type RunFunc func(ctx context.Context, sess *agent.Session, userText string)

// CompactFunc runs forced compaction. Returns (messagesBefore, messagesAfter, error).
type CompactFunc func(ctx context.Context, sess *agent.Session) (int, int, error)

// AppState tracks what the TUI is doing.
type AppState int

const (
	stateInput AppState = iota
	stateStreaming
	statePermission
	stateGovernance
	stateSessionPicker
)

// Model is the central BubbleTea model for the agent TUI.
type Model struct {
	state   AppState
	session *agent.Session

	// Chat content
	messages  []viewMessage
	streamBuf *strings.Builder
	thinkBuf  *strings.Builder // reasoning/thinking text accumulator
	errMsg    string

	// Lemniscate phase
	currentPhase agent.Phase // current lemniscate phase (empty = no lemniscate)
	phaseName    string      // display name (e.g., "haft-framer")
	phaseVerb    string      // current status verb (picked once per phase activation)
	verbCounter  int         // increments each phase activation, selects word from pool
	phaseReply   chan<- bool // phase pause reply: true=proceed, false=discuss

	// Active cycle tracking (from CycleUpdateMsg)
	cycleID      string
	problemRef   string
	portfolioRef string
	decisionRef  string
	cycleStatus  agent.CycleStatus
	cycleREff    float64

	// Token tracking
	tokensUsed  int
	tokensLimit int

	// Subagent tracking
	activeSubagents int // count of running subagents

	// Mode toggles
	autoApprove bool // Ctrl+Y: auto-approve tool permissions
	prefixMode  bool // Ctrl+S prefix: next key selects action
	quitConfirm bool // Ctrl+C pressed once, waiting for confirm

	// Command palette
	palette    CommandPalette
	filePicker *FilePicker // @ file mention completion

	// Message queue (user types during streaming)
	pendingMessages []string

	// Paste attachments (large pastes → files instead of inline)
	attachments []pasteAttachment
	pasteIdx    int

	// Permission
	permToolName, permArgs string
	permReply              chan<- bool

	// Phase pause
	govRationale string // summary text for phase transition overlay

	// Components
	input       textarea.Model
	chatList    ChatList
	spinner     spinner.Model
	picker      list.Model // session picker (for /resume)
	gliderTick  int        // glider animation frame (advances slowly)
	spinnerTick int        // raw spinner tick counter (fast, ~80ms)

	// Infra
	bus             *Bus
	runFn           RunFunc
	compactFn       CompactFunc
	cancel          context.CancelFunc
	sessionStore    session.SessionStore
	sessionMsgStore session.MessageStore
	cycleStore      session.CycleStore

	// Layout
	width, height int
	styles        Styles

	// Status bar notification (transient, clears after timeout)
	notification string

	initialGoal string
}

func New(
	session *agent.Session,
	runFn RunFunc,
	bus *Bus,
	initialGoal string,
	sessStore session.SessionStore,
	msgStore session.MessageStore,
	compactFn CompactFunc,
	cycleStore session.CycleStore,
	projectRoot string,
) Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.ShowLineNumbers = false
	ta.SetHeight(1)
	ta.DynamicHeight = true
	ta.MinHeight = 1
	ta.MaxHeight = 8
	ta.CharLimit = 0
	ta.Focus()
	ta.SetPromptFunc(2, func(_ textarea.PromptInfo) string {
		return "│ "
	})

	// Clean textarea styling — no cursor line highlight, no base padding
	s := ta.Styles()
	s.Focused.CursorLine = lipgloss.NewStyle()
	s.Focused.Base = lipgloss.NewStyle()
	s.Blurred.Base = lipgloss.NewStyle()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	s.Blurred.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	s.Focused.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	s.Blurred.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	s.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	s.Blurred.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	ta.SetStyles(s)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))

	return Model{
		state:           stateInput,
		session:         session,
		bus:             bus,
		runFn:           runFn,
		compactFn:       compactFn,
		sessionStore:    sessStore,
		sessionMsgStore: msgStore,
		cycleStore:      cycleStore,
		styles:          DefaultStyles(),
		initialGoal:     initialGoal,
		streamBuf:       &strings.Builder{},
		thinkBuf:        &strings.Builder{},
		input:           ta,
		spinner:         sp,
		filePicker:      NewFilePicker(projectRoot),
	}
}

// ---------------------------------------------------------------------------
// BubbleTea interface
// ---------------------------------------------------------------------------

func (m Model) waitForBus() tea.Cmd {
	bus := m.bus
	return func() tea.Msg { return bus.Listen() }
}

type submitMsg struct{ text string }

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.waitForBus(),
		m.input.Focus(),
		m.spinner.Tick,
	}
	if m.initialGoal != "" {
		goal := m.initialGoal
		cmds = append(cmds, func() tea.Msg { return submitMsg{text: goal} })
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeComponents()
		return m, nil

	case tea.MouseWheelMsg:
		m.chatList.HandleMouseWheel(msg.Button)
		return m, nil

	case tea.MouseClickMsg:
		_, _, chatH := m.layoutBlocks()
		if msg.Y < chatH {
			cmd := m.chatList.HandleMouseDown(msg.X, msg.Y)
			return m, cmd
		}
		return m, nil

	case tea.MouseMotionMsg:
		_, _, chatH := m.layoutBlocks()
		if msg.Y < chatH {
			m.chatList.HandleMouseDrag(msg.X, msg.Y)
		}
		return m, nil

	case tea.MouseReleaseMsg:
		_, _, chatH := m.layoutBlocks()
		if msg.Y < chatH {
			cmd := m.chatList.HandleMouseUp(msg.X, msg.Y)
			return m, cmd
		}
		return m, nil

	case CopySelectionMsg:
		if msg.Text != "" {
			m.notification = "copied to clipboard"
			return m, tea.Batch(
				CopyToClipboard(msg.Text),
				tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
					return clearNotificationMsg{}
				}),
			)
		}
		return m, nil

	case clearNotificationMsg:
		m.notification = ""
		return m, nil

	case clearQuitConfirmMsg:
		m.quitConfirm = false
		if m.notification == "Press Ctrl+C again to quit, or keep working" {
			m.notification = ""
		}
		return m, nil

	case compactDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("compact failed: %s", msg.err)
			m.notification = ""
		} else {
			m.notification = fmt.Sprintf("compacted %d → %d messages", msg.before, msg.after)
		}
		return m, tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
			return clearNotificationMsg{}
		})

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.PasteMsg:
		if m.state == stateInput || m.state == stateStreaming {
			content := msg.Content
			// Large paste → attachment (like Crush: >10 newlines or >1000 chars)
			if isLargePaste(content) {
				m.pasteIdx++
				name := fmt.Sprintf("paste_%d.txt", m.pasteIdx)
				m.attachments = append(m.attachments, pasteAttachment{Name: name, Content: content})
				m.notification = fmt.Sprintf("📎 %s attached (%d chars)", name, len([]rune(content)))
			} else {
				m.input.SetValue(m.input.Value() + content)
				m.resizeComponents()
			}
		}
		return m, nil

	case submitMsg:
		return m.handleSubmit(msg.text)

	case spinner.TickMsg:
		if m.state == stateStreaming || m.state == statePermission || m.state == stateGovernance || m.notification != "" {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			m.spinnerTick++
			// Advance animation every 5 spinner ticks (~400ms per frame)
			if m.spinnerTick%5 == 0 {
				m.gliderTick++
			}
			m.refreshChat()
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	// --- Bus events ---
	case ThinkingDeltaMsg:
		m.appendStreamingThinking(msg.Text)
		m.refreshChat()
		return m, m.waitForBus()

	case StreamDeltaMsg:
		m.appendStreamingText(msg.Text)
		m.errMsg = ""
		m.refreshChat()
		return m, m.waitForBus()

	case StreamDoneMsg:
		m.finalizeStreamMessage(msg.Message)
		if len(msg.Message.ToolCalls()) == 0 {
			m.state = stateInput
			cmds = append(cmds, m.input.Focus())
		}
		m.refreshChat()
		return m, tea.Batch(append(cmds, m.waitForBus())...)

	case ToolStartMsg:
		if msg.SubagentID != "" {
			// Child tool call — nest under the spawn tool
			m.addChildTool(msg.SubagentID, viewTool{
				CallID: msg.ToolCallID, Name: msg.ToolName, Args: msg.Args, Running: true,
			})
		} else {
			m.ensureAssistantMessage()
			last := &m.messages[len(m.messages)-1]
			last.Tools = append(last.Tools, viewTool{
				CallID: msg.ToolCallID, Name: msg.ToolName, Args: msg.Args, Running: true,
			})
		}
		m.refreshChat()
		return m, m.waitForBus()

	case ToolDoneMsg:
		if msg.SubagentID != "" {
			m.completeChildTool(msg.SubagentID, msg.ToolCallID, msg.Output, msg.IsError)
		} else {
			m.completeToolCall(msg.ToolCallID, msg.ToolName, msg.Output, msg.IsError)
		}
		m.refreshChat()
		return m, m.waitForBus()

	case SubagentStartMsg:
		m.activeSubagents++
		m.tagSpawnTool(msg.SubagentID)
		m.refreshChat()
		return m, m.waitForBus()

	case SubagentDoneMsg:
		if m.activeSubagents > 0 {
			m.activeSubagents--
		}
		m.completeSpawnTool(msg.SubagentID, msg.Summary, msg.IsError)
		m.refreshChat()
		return m, m.waitForBus()

	case PermissionAskMsg:
		if m.autoApprove {
			// Yolo mode: auto-approve without showing dialog
			msg.Reply <- true
			return m, m.waitForBus()
		}
		m.state = statePermission
		m.permToolName = msg.ToolName
		m.permArgs = msg.Args
		m.permReply = msg.Reply
		m.refreshChat()
		return m, nil

	case ErrorMsg:
		m.errMsg = msg.Err.Error()
		m.state = stateInput
		cmds = append(cmds, m.input.Focus())
		m.refreshChat()
		return m, tea.Batch(append(cmds, m.waitForBus())...)

	case SessionTitleMsg:
		m.session.Title = msg.Title
		return m, m.waitForBus()

	case TokenUpdateMsg:
		m.tokensUsed = msg.Used
		m.tokensLimit = msg.Limit
		return m, m.waitForBus()

	case CycleUpdateMsg:
		m.cycleID = msg.CycleID
		m.problemRef = msg.ProblemRef
		m.portfolioRef = msg.PortfolioRef
		m.decisionRef = msg.DecisionRef
		m.cycleStatus = msg.Status
		m.cycleREff = msg.REff
		m.refreshChat()
		return m, m.waitForBus()

	case PhaseChangeMsg:
		// Finalize any in-progress streaming from the previous phase
		if m.streamBuf.Len() > 0 || m.thinkBuf.Len() > 0 {
			m.finalizeStream()
		}
		m.currentPhase = msg.To
		m.phaseName = msg.Name
		m.verbCounter++
		m.phaseVerb = m.pickVerb(msg.To)
		m.refreshChat()
		return m, m.waitForBus()

	case PhasePauseMsg:
		m.state = stateGovernance // reuse governance state — Enter to proceed
		m.govRationale = msg.Summary
		m.phaseReply = msg.Reply
		m.refreshChat()
		return m, m.spinner.Tick

	case CoordinatorDoneMsg:
		m.state = stateInput
		cmds = append(cmds, m.input.Focus())
		m.refreshChat()

		// Process queued messages (user typed during streaming)
		if len(m.pendingMessages) > 0 {
			next := m.pendingMessages[0]
			m.pendingMessages = m.pendingMessages[1:]
			return m.handleSubmit(next)
		}
		return m, tea.Batch(cmds...)
	}

	// Forward to components
	if m.state == stateInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

func (m *Model) resizeComponents() {
	m.input.SetWidth(max(1, m.width-4))
	_, _, chatH := m.layoutBlocks()
	sizeChanged := m.chatList.width != m.width || m.chatList.height != chatH
	m.chatList.SetSize(m.width, chatH)
	// Only rebuild items if viewport size actually changed.
	// Keystroke in textarea doesn't change chat content — skip expensive rebuild.
	if sizeChanged {
		m.refreshChat()
	}
}

func (m Model) buildInputBlock() string {
	borderLine := m.styles.InputBorder.Render(strings.Repeat("━", m.width))
	if m.state == stateInput || m.state == stateStreaming {
		var parts []string

		// Attachment chips above input
		if len(m.attachments) > 0 {
			chipStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("48")).
				Bold(true)
			var chips []string
			for _, att := range m.attachments {
				chips = append(chips, chipStyle.Render(" 📎 "+att.Name+" "))
			}
			parts = append(parts, strings.Join(chips, " "))
		}

		parts = append(parts, m.input.View())

		if len(m.pendingMessages) > 0 {
			parts = append(parts, m.styles.Dim.Render(fmt.Sprintf("  (%d queued)", len(m.pendingMessages))))
		}

		return borderLine + "\n" + strings.Join(parts, "\n") + "\n" + borderLine
	}
	return borderLine + "\n" + m.styles.Dim.Render("│") + "\n" + borderLine
}

func (m Model) layoutBlocks() (string, string, int) {
	inputBlock := m.buildInputBlock()
	statusBlock := m.renderStatusBlock()
	inputH := lipgloss.Height(inputBlock)
	statusH := lipgloss.Height(statusBlock)
	chatH := m.height - inputH - statusH - 2 // separator rows between chat/input and input/status
	if chatH < 1 {
		chatH = 1
	}
	return inputBlock, statusBlock, chatH
}

func (m *Model) refreshChat() {
	items := m.buildChatItems()
	m.chatList.SetItems(items)
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()

	// Global: Ctrl+C — first press cancels/shows quit confirm, second quits
	if key.Mod == tea.ModCtrl && key.Code == 'c' {
		if m.state == statePermission && m.permReply != nil {
			m.permReply <- false
		}
		if m.state == stateGovernance && m.phaseReply != nil {
			m.phaseReply <- false
			m.phaseReply = nil
		}
		if m.cancel != nil {
			m.cancel()
		}
		if m.state == stateInput {
			if m.quitConfirm {
				return m, tea.Quit
			}
			m.quitConfirm = true
			m.notification = "Press Ctrl+C again to quit, or keep working"
			return m, tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
				return clearQuitConfirmMsg{}
			})
		}
		m.state = stateInput
		return m, m.input.Focus()
	}
	// Any other key clears quit confirm
	if m.quitConfirm {
		m.quitConfirm = false
		m.notification = ""
	}
	// Global: Ctrl+Q toggles interaction mode (symbiotic ↔ autonomous)
	if key.Mod == tea.ModCtrl && key.Code == 'q' {
		if m.session.Interaction == agent.InteractionAutonomous {
			m.session.Interaction = agent.InteractionSymbiotic
		} else {
			m.session.Interaction = agent.InteractionAutonomous
		}
		return m, nil
	}
	// Global: Ctrl+Y toggles tool auto-approve (yolo)
	if key.Mod == tea.ModCtrl && key.Code == 'y' {
		m.autoApprove = !m.autoApprove
		return m, nil
	}
	if key.Mod == tea.ModCtrl && key.Code == 'd' {
		return m, tea.Quit
	}
	// Ctrl+S prefix mode: Ctrl+S then next key selects action
	if key.Mod == tea.ModCtrl && key.Code == 's' {
		m.prefixMode = true
		m.notification = "C-s ..."
		return m, nil
	}
	if m.prefixMode {
		m.prefixMode = false
		m.notification = ""
		switch key.Code {
		case 'i': // Ctrl+S, i → copy session ID
			m.notification = "session " + m.session.ID
			return m, tea.Batch(
				CopyToClipboard(m.session.ID),
				tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
					return clearNotificationMsg{}
				}),
			)
		default:
			return m, nil
		}
	}
	// Global: Ctrl+O toggles subagent expand/collapse
	if key.Mod == tea.ModCtrl && key.Code == 'o' {
		m.toggleSubagentExpand()
		m.refreshChat()
		return m, nil
	}

	// Session picker gets all input when active
	if m.state == stateSessionPicker {
		return m.handleSessionPickerKey(msg)
	}

	// Scroll: direct method calls work in ALL states.
	// PgUp/PgDown for page scroll, Shift+Up/Down for line scroll.
	switch {
	case key.Code == tea.KeyPgUp:
		m.chatList.PageUp()
		return m, nil
	case key.Code == tea.KeyPgDown:
		m.chatList.PageDown()
		return m, nil
	case key.Mod == tea.ModShift && key.Code == tea.KeyUp:
		m.chatList.ScrollBy(-3)
		return m, nil
	case key.Mod == tea.ModShift && key.Code == tea.KeyDown:
		m.chatList.ScrollBy(3)
		return m, nil
	}

	switch m.state {
	case stateInput:
		// Command palette: intercept keys when visible
		if m.palette.Visible() {
			switch {
			case key.Code == tea.KeyUp || key.Code == 'k':
				m.palette.MoveUp()
				return m, nil
			case key.Code == tea.KeyDown || key.Code == 'j':
				m.palette.MoveDown()
				return m, nil
			case key.Code == tea.KeyTab:
				// Tab: fill input with selected command
				if sel := m.palette.Selected(); sel != "" {
					m.input.SetValue(sel)
					m.input.CursorEnd()
					m.palette.Update(sel)
				}
				return m, nil
			case key.Code == tea.KeyEnter && key.Mod == 0:
				// Enter: execute selected command
				sel := m.palette.Selected()
				if sel == "" {
					sel = strings.TrimSpace(m.input.Value())
				}
				if sel != "" {
					m.input.SetValue("")
					m.palette.Update("")
					return m.handleSubmit(sel)
				}
				return m, nil
			case key.Code == tea.KeyEscape:
				m.input.SetValue("")
				m.palette.Update("")
				return m, nil
			}
		}

		// File picker: intercept keys when visible (@ mention)
		if m.filePicker != nil && m.filePicker.Visible() {
			switch {
			case key.Code == tea.KeyUp || key.Code == 'k':
				m.filePicker.MoveUp()
				return m, nil
			case key.Code == tea.KeyDown || key.Code == 'j':
				m.filePicker.MoveDown()
				return m, nil
			case key.Code == tea.KeyTab || (key.Code == tea.KeyEnter && key.Mod == 0):
				// Select file → attach content
				if sel := m.filePicker.Selected(); sel != "" {
					m.attachFile(sel)
					// Remove @query from input
					val := m.input.Value()
					if atIdx := strings.LastIndex(val, "@"); atIdx >= 0 {
						m.input.SetValue(val[:atIdx])
					}
				}
				return m, nil
			case key.Code == tea.KeyEscape:
				m.filePicker.Update("") // close
				return m, nil
			}
		}

		// Ctrl+J inserts newline (like Claude Code / Codex)
		if key.Mod == tea.ModCtrl && key.Code == 'j' {
			m.input.InsertString("\n")
			return m, nil
		}

		// Submit on Enter (no modifier)
		if key.Code == tea.KeyEnter && key.Mod == 0 {
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			m.input.SetValue("")
			return m.handleSubmit(text)
		}

		// Forward everything else to textarea
		prevInputH := m.input.Height()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		// Update command palette + file picker filter from current input
		m.palette.Update(m.input.Value())
		if m.filePicker != nil {
			m.filePicker.Update(m.input.Value())
		}
		// Only resize if textarea height changed (multiline expansion)
		if m.input.Height() != prevInputH {
			m.resizeComponents()
		}
		return m, cmd

	case stateStreaming:
		return m, nil

	case statePermission:
		switch key.Text {
		case "y", "Y":
			m.permReply <- true
			m.state = stateStreaming
			return m, tea.Batch(m.waitForBus(), m.spinner.Tick)
		case "n", "N":
			m.permReply <- false
			m.state = stateStreaming
			return m, tea.Batch(m.waitForBus(), m.spinner.Tick)
		}

	case stateGovernance:
		switch key.Text {
		case "y", "Y":
			if m.phaseReply != nil {
				m.phaseReply <- true
				m.phaseReply = nil
			}
			m.state = stateStreaming
			return m, tea.Batch(m.waitForBus(), m.spinner.Tick)
		case "n", "N":
			if m.phaseReply != nil {
				m.phaseReply <- false
				m.phaseReply = nil
			}
			m.state = stateInput
			return m, m.input.Focus()
		}
		if key.Code == tea.KeyEnter {
			if m.phaseReply != nil {
				m.phaseReply <- true
				m.phaseReply = nil
			}
			m.state = stateStreaming
			return m, tea.Batch(m.waitForBus(), m.spinner.Tick)
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Submit
// ---------------------------------------------------------------------------

func (m Model) handleSubmit(text string) (tea.Model, tea.Cmd) {
	// Slash commands
	if strings.HasPrefix(text, "/") {
		return m.handleSlashCommand(text)
	}

	// Prepend paste attachments as inline content (like Crush's <file> tags)
	if len(m.attachments) > 0 {
		var prefix strings.Builder
		for _, att := range m.attachments {
			prefix.WriteString(fmt.Sprintf("<file path=%q>\n%s\n</file>\n\n", att.Name, att.Content))
		}
		text = prefix.String() + text
		m.attachments = nil
		m.notification = ""
	}

	if m.streamBuf == nil {
		m.streamBuf = &strings.Builder{}
	}
	m.messages = append(m.messages, viewMessage{Role: agent.RoleUser, Text: text})
	m.state = stateStreaming
	m.streamBuf.Reset()
	m.thinkBuf.Reset()
	m.errMsg = ""
	m.verbCounter++
	m.phaseVerb = m.pickVerb(m.currentPhase)
	m.input.Blur()
	m.refreshChat()

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	runFn := m.runFn
	sess := m.session

	return m, tea.Batch(
		m.waitForBus(),
		m.spinner.Tick,
		func() tea.Msg {
			runFn(ctx, sess, text)
			return nil
		},
	)
}

// ---------------------------------------------------------------------------
// Slash commands
// ---------------------------------------------------------------------------

func (m Model) handleSlashCommand(text string) (tea.Model, tea.Cmd) {
	cmd := strings.TrimSpace(strings.ToLower(text))

	switch cmd {
	case "/resume", "/sessions":
		return m.openSessionPicker()
	case "/compact":
		if m.compactFn == nil {
			m.errMsg = "compaction not available"
			return m, nil
		}
		m.notification = "compacting..."
		compactFn := m.compactFn
		sess := m.session
		return m, func() tea.Msg {
			before, after, err := compactFn(context.Background(), sess)
			return compactDoneMsg{before: before, after: after, err: err}
		}
	case "/help":
		var names []string
		for _, sc := range slashCommands {
			names = append(names, "/"+sc.Name)
		}
		m.errMsg = "Commands: " + strings.Join(names, ", ")
		return m, nil
	case "/frame", "/explore", "/decide", "/measure", "/status",
		"/reason", "/note", "/search", "/compare", "/problems",
		"/refresh", "/char":
		// Pass through to agent as user message (strip slash to avoid recursion)
		return m.handleSubmit(strings.TrimPrefix(text, "/"))
	default:
		m.errMsg = fmt.Sprintf("Unknown command: %s. Type / to see available commands", cmd)
		return m, nil
	}
}

func (m Model) openSessionPicker() (tea.Model, tea.Cmd) {
	picker, err := buildSessionPicker(
		context.Background(),
		m.sessionStore,
		m.sessionMsgStore,
		m.width,
		m.height-6, // leave space for borders
	)
	if err != nil {
		m.errMsg = fmt.Sprintf("Failed to load sessions: %s", err)
		return m, nil
	}
	m.picker = picker
	m.state = stateSessionPicker
	m.input.Blur()
	return m, nil
}

// ---------------------------------------------------------------------------
// State management
// ---------------------------------------------------------------------------

func (m *Model) finalizeStream() {
	m.finalizeStreamText(m.streamBuf.String(), m.thinkBuf.String())
}

func (m *Model) finalizeStreamMessage(msg agent.Message) {
	m.finalizeStreamText(msg.Text(), m.thinkBuf.String())
}

func (m *Model) finalizeStreamText(text, thinking string) {
	m.streamBuf.Reset()
	m.thinkBuf.Reset()

	if text == "" && thinking == "" {
		return
	}

	last := m.ensureAssistantMessage()
	last.Text = text
	last.Thinking = thinking
}

func (m *Model) appendStreamingText(delta string) {
	m.streamBuf.WriteString(delta)
	last := m.ensureAssistantMessage()
	last.Text += delta
}

func (m *Model) appendStreamingThinking(delta string) {
	m.thinkBuf.WriteString(delta)
	last := m.ensureAssistantMessage()
	last.Thinking += delta
}

func (m *Model) ensureAssistantMessage() *viewMessage {
	last := m.lastAssistantInPhase()
	if last != nil && !last.hasCompletedTools() {
		return last
	}
	m.messages = append(m.messages, viewMessage{
		Role:  agent.RoleAssistant,
		Phase: m.currentPhase,
	})
	return &m.messages[len(m.messages)-1]
}

// lastAssistantInPhase returns the last assistant message that belongs
// to the current phase. Returns nil if no message exists for this phase,
// which forces a new message to be created.
func (m *Model) lastAssistantInPhase() *viewMessage {
	if n := len(m.messages); n > 0 {
		last := &m.messages[n-1]
		if last.Role == agent.RoleAssistant && last.Phase == m.currentPhase {
			return last
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Subagent tool routing
// ---------------------------------------------------------------------------

// tagSpawnTool finds the most recent spawn_agent tool and tags it with the SubagentID.
func (m *Model) tagSpawnTool(subagentID string) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		for j := range m.messages[i].Tools {
			t := &m.messages[i].Tools[j]
			if t.Name == "spawn_agent" && t.SubagentID == "" {
				t.SubagentID = subagentID
				return
			}
		}
	}
}

// addChildTool nests a tool call under the spawn_agent tool with the given SubagentID.
func (m *Model) addChildTool(subagentID string, child viewTool) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		for j := range m.messages[i].Tools {
			if m.messages[i].Tools[j].SubagentID == subagentID {
				m.messages[i].Tools[j].Children = append(m.messages[i].Tools[j].Children, child)
				return
			}
		}
	}
}

// completeChildTool marks a nested child tool call as done.
func (m *Model) completeChildTool(subagentID, callID, output string, isError bool) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		for j := range m.messages[i].Tools {
			if m.messages[i].Tools[j].SubagentID != subagentID {
				continue
			}
			for k := range m.messages[i].Tools[j].Children {
				if m.messages[i].Tools[j].Children[k].CallID == callID {
					m.messages[i].Tools[j].Children[k].Output = output
					m.messages[i].Tools[j].Children[k].IsError = isError
					m.messages[i].Tools[j].Children[k].Running = false
					return
				}
			}
		}
	}
}

// toggleSubagentExpand toggles expanded state on all subagent tool blocks.
func (m *Model) toggleSubagentExpand() {
	for i := range m.messages {
		for j := range m.messages[i].Tools {
			if m.messages[i].Tools[j].SubagentID != "" {
				m.messages[i].Tools[j].Expanded = !m.messages[i].Tools[j].Expanded
			}
		}
	}
}

// completeSpawnTool marks the spawn_agent tool as complete with the subagent's summary.
func (m *Model) completeSpawnTool(subagentID, summary string, isError bool) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		for j := range m.messages[i].Tools {
			if m.messages[i].Tools[j].SubagentID == subagentID {
				m.messages[i].Tools[j].Running = false
				m.messages[i].Tools[j].Output = summary
				m.messages[i].Tools[j].IsError = isError
				return
			}
		}
	}
}

func (m *Model) completeToolCall(callID, name, output string, isError bool) {
	if len(m.messages) == 0 {
		return
	}
	last := &m.messages[len(m.messages)-1]
	// Match by callID first (unique), fall back to name+running
	for i := range last.Tools {
		if last.Tools[i].CallID == callID {
			last.Tools[i].Output = output
			last.Tools[i].IsError = isError
			last.Tools[i].Running = false
			return
		}
	}
	for i := range last.Tools {
		if last.Tools[i].Name == name && last.Tools[i].Running {
			last.Tools[i].Output = output
			last.Tools[i].IsError = isError
			last.Tools[i].Running = false
			return
		}
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m Model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		v := tea.NewView("Initializing...")
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		// Mouse capture enables scroll. Hold Shift in tmux for text selection.
		return v
	}

	// Session picker overlay replaces the entire view
	if m.state == stateSessionPicker {
		content := m.picker.View()
		v := tea.NewView(content)
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		// Mouse capture enables scroll. Hold Shift in tmux for text selection.
		v.WindowTitle = "haft — resume session"
		return v
	}

	inputBlock, statusBlock, chatH := m.layoutBlocks()

	// Ensure chatList has correct size for this render
	if m.chatList.height != chatH || m.chatList.width != m.width {
		m.chatList.SetSize(m.width, chatH)
	}

	canvas := uv.NewScreenBuffer(m.width, m.height)
	canvas.Method = ansi.GraphemeWidth

	chatView := m.chatList.View()
	drawBlock(&canvas, 0, 0, m.width, chatH, chatView)
	if m.palette.Visible() {
		paletteBox := m.palette.Render(m.width, m.styles)
		drawOverlayBottom(&canvas, 0, 0, m.width, chatH, paletteBox)
	} else if m.filePicker != nil && m.filePicker.Visible() {
		pickerBox := m.filePicker.Render(m.width, m.styles)
		drawOverlayBottom(&canvas, 0, 0, m.width, chatH, pickerBox)
	}

	inputY := chatH + 1
	drawBlock(&canvas, 0, inputY, m.width, lipgloss.Height(inputBlock), inputBlock)

	statusY := inputY + lipgloss.Height(inputBlock) + 1
	drawBlock(&canvas, 0, statusY, m.width, lipgloss.Height(statusBlock), statusBlock)

	var v tea.View
	v.SetContent(canvas.Render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = fmt.Sprintf("haft — %s", m.session.Model)

	if m.state == stateInput {
		if c := m.input.Cursor(); c != nil {
			c.Y += inputY
			v.Cursor = c
		}
	}

	return v
}

func drawBlock(buf *uv.ScreenBuffer, x, y, width, height int, content string) {
	if width <= 0 || height <= 0 || content == "" {
		return
	}
	area := uv.Rect(x, y, x+width, y+height)
	styled := uv.NewStyledString(content)
	styled.Wrap = true
	styled.Draw(*buf, area)
}

func drawOverlayBottom(buf *uv.ScreenBuffer, x, y, width, height int, overlay string) {
	if width <= 0 || height <= 0 || overlay == "" {
		return
	}
	overlayH := strings.Count(overlay, "\n") + 1
	if overlayH > height {
		overlayH = height
	}
	startY := y + height - overlayH
	drawBlock(buf, x, startY, width, overlayH, overlay)
}

func (m Model) renderStatusBlock() string {
	innerWidth := max(1, m.width-4)
	sid := m.session.ID
	if len(sid) > 8 {
		sid = sid[:8]
	}

	anim := m.selectAnimation()
	glider := AnimationCells(anim, m.gliderTick)
	var stateText string

	switch m.state {
	case stateInput:
		stateText = m.styles.StatusState.Render("ready")
	case stateStreaming:
		verb := m.phaseVerb
		if verb == "" {
			verb = "reasoning"
		}
		stateText = m.scanText(verb)
	case statePermission:
		stateText = m.styles.PermTitle.Render("permission")
	case stateGovernance:
		stateText = m.styles.PermTitle.Render("phase transition")
	}

	// Mode indicators
	modeIndicator := ""
	if m.session.Interaction == agent.InteractionAutonomous {
		modeIndicator += m.styles.ToolRunning.Render("⚡auto") + " "
	}
	if m.autoApprove {
		modeIndicator += m.styles.ErrorText.Render("⚠yolo") + " "
	}

	title := fmt.Sprintf("%s %s%s",
		m.styles.HeaderBar.Render("haft"),
		modeIndicator,
		stateText,
	)

	// Cycle pipeline display
	cycleInfo := m.renderCycleInfo()

	scrollHint := ""
	if !m.chatList.AtBottom() {
		pct := int(m.chatList.ScrollPercent() * 100)
		scrollHint = m.styles.Dim.Render(fmt.Sprintf(" ↑%d%% ", pct))
	}

	// Token counter
	tokenInfo := ""
	if m.tokensLimit > 0 {
		usedK := m.tokensUsed / 1000
		limitK := m.tokensLimit / 1000
		tokenInfo = fmt.Sprintf(" │ %dk/%dk", usedK, limitK)
	}

	// Subagent indicator
	subagentIndicator := ""
	if m.activeSubagents > 0 {
		subagentIndicator = m.styles.ToolRunning.Render(fmt.Sprintf(" %d ⇶ agents", m.activeSubagents))
	}

	meta := fmt.Sprintf("%s │ %s │ t%d%s%s",
		m.styles.StatusModel.Render(m.session.Model),
		m.styles.Dim.Render(sid),
		len(m.messages),
		m.styles.Dim.Render(tokenInfo),
		subagentIndicator,
	)

	rows := m.renderGliderRows(glider)
	line0Left := rows[0]
	if cycleInfo != "" {
		line0Left = rows[0] + "  " + cycleInfo
	}
	line0 := padStatusRow(line0Left, scrollHint, innerWidth)
	line1 := padStatusRow(rows[1]+"  "+title, "", innerWidth)
	line2 := padStatusRow(rows[2], meta, innerWidth)

	// Inline notification replaces the title in line1 when active
	if m.notification != "" {
		notif := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("48")).
			Render(" " + m.notification + " ")
		line1 = padStatusRow(rows[1]+"  "+notif, "", innerWidth)
	}

	return "  " + line0 + "  \n" +
		"  " + line1 + "  \n" +
		"  " + line2 + "  "
}

func padStatusRow(left string, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) renderGliderRows(glider [3][3]bool) [3]string {
	var rows [3]string
	for i, row := range glider {
		rows[i] = m.renderGliderRow(row)
	}
	return rows
}

func (m Model) renderGliderRow(row [3]bool) string {
	cells := make([]string, 0, len(row))
	for _, alive := range row {
		if alive {
			cells = append(cells, m.styles.GliderLive.Render("●"))
			continue
		}
		cells = append(cells, m.styles.GliderDead.Render("○"))
	}
	return strings.Join(cells, " ")
}

// renderCycleInfo renders the cycle pipeline for the status bar top line.
// Shows: Working on: prob-001 → sol-001 → deciding...
func (m Model) renderCycleInfo() string {
	if m.cycleID == "" {
		return ""
	}

	accent := m.styles.StatusAccent
	dim := m.styles.Dim

	var refs []string

	if m.problemRef != "" {
		refs = append(refs, accent.Render(truncRef(m.problemRef)))
	}
	if m.portfolioRef != "" {
		refs = append(refs, accent.Render(truncRef(m.portfolioRef)))
	} else if m.problemRef != "" && m.currentPhase == agent.PhaseExplorer {
		refs = append(refs, dim.Render("exploring…"))
	}
	if m.decisionRef != "" {
		refs = append(refs, accent.Render(truncRef(m.decisionRef)))
	} else if m.portfolioRef != "" && m.currentPhase == agent.PhaseDecider {
		refs = append(refs, dim.Render("deciding…"))
	}

	if m.cycleStatus == agent.CycleComplete {
		rEffLabel := fmt.Sprintf("✓ R:%.1f", m.cycleREff)
		if m.cycleREff >= 0.7 {
			refs = append(refs, m.styles.ToolDone.Render(rEffLabel))
		} else if m.cycleREff >= 0.4 {
			refs = append(refs, m.styles.Dim.Render(rEffLabel))
		} else {
			refs = append(refs, m.styles.ErrorText.Render(rEffLabel))
		}
	}

	if len(refs) == 0 {
		return ""
	}

	pipeline := strings.Join(refs, dim.Render(" → "))
	return dim.Render("Working on: ") + pipeline
}

// truncRef shortens an artifact ref for display (prob-20260329-001 → prob-001).
func truncRef(ref string) string {
	// refs look like "prob-20260329-001" — show "prob-001" (kind + seq)
	parts := strings.Split(ref, "-")
	if len(parts) >= 3 {
		return parts[0] + "-" + parts[len(parts)-1]
	}
	if len(ref) > 12 {
		return ref[:12]
	}
	return ref
}

// selectAnimation picks the animation pattern based on current TUI state.
func (m Model) selectAnimation() Animation {
	switch m.state {
	case stateStreaming:
		if m.activeSubagents > 0 {
			return AnimOrbit // dot circling — agents working
		}
		return AnimGlider // Conway's glider — LLM thinking
	case statePermission, stateGovernance:
		return AnimPulse // center radiates — waiting for user
	default:
		if strings.Contains(m.notification, "compact") {
			return AnimConverge // corners→center — compaction
		}
		return AnimStatic // frozen — idle
	}
}

// pickVerb selects a status verb for a phase activation.
// Called once when a phase starts or streaming begins — the word holds for the entire run.
// Uses verbCounter to cycle through the pool so each activation gets a different word.
// ---------------------------------------------------------------------------
// File attachment (@ mention + paste)
// ---------------------------------------------------------------------------

// attachFile reads a file and adds it as an attachment.
func (m *Model) attachFile(relPath string) {
	if m.filePicker == nil {
		return
	}
	absPath := filepath.Join(m.filePicker.projectRoot, relPath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		m.errMsg = fmt.Sprintf("Failed to read %s: %s", relPath, err)
		return
	}

	m.attachments = append(m.attachments, pasteAttachment{
		Name:    relPath,
		Content: string(content),
	})
	m.notification = fmt.Sprintf("📎 %s attached", relPath)
}

// ---------------------------------------------------------------------------
// Paste attachments
// ---------------------------------------------------------------------------

// pasteAttachment holds large pasted text as a file-like attachment.
type pasteAttachment struct {
	Name    string // "paste_1.txt"
	Content string
}

const (
	pasteLinesThreshold = 10   // >10 newlines → attachment
	pasteCharsThreshold = 1000 // >1000 chars → attachment
)

// isLargePaste returns true if pasted content should become an attachment.
func isLargePaste(content string) bool {
	if len([]rune(content)) > pasteCharsThreshold {
		return true
	}
	lines := 0
	for _, c := range content {
		if c == '\n' {
			lines++
			if lines > pasteLinesThreshold {
				return true
			}
		}
	}
	return false
}

func (m Model) pickVerb(phase agent.Phase) string {
	pools := map[agent.Phase][]string{
		agent.PhaseFramer: {
			"characterizing", "diagnosing", "scrying", "probing", "bounding",
			"dissecting", "triangulating", "divining", "deciphering", "unraveling",
			"enumerating", "fingerprinting", "reconnoitering", "surveying", "tracing",
		},
		agent.PhaseExplorer: {
			"abducting", "conjuring", "diverging", "conjecturing", "transmuting",
			"generating", "branching", "forking", "invoking", "sublimating",
			"reifying", "instantiating", "propagating", "permuting", "extrapolating",
		},
		agent.PhaseDecider: {
			"evaluating", "distilling", "selecting", "resolving", "calibrating",
			"arbitrating", "converging", "normalizing", "binding", "precipitating",
			"unifying", "reducing", "bisecting", "quantifying", "adjudicating",
		},
		agent.PhaseWorker: {
			"constructing", "forging", "composing", "deriving", "weaving",
			"synthesizing", "inscribing", "patching", "shimming", "bootstrapping",
			"marshaling", "pipelining", "refactoring", "splicing", "compiling",
		},
		agent.PhaseMeasure: {
			"validating", "assaying", "inducing", "corroborating", "verifying",
			"checksumming", "benchmarking", "falsifying", "proving", "auditing",
			"regulating", "certifying", "inspecting", "profiling", "attesting",
		},
	}

	pool, ok := pools[phase]
	if !ok {
		pool = []string{
			"reasoning", "grokking", "traversing", "computing", "spelunking",
			"modeling", "channeling", "attuning", "iterating", "recursing",
			"integrating", "consolidating", "tunneling", "buffering", "multiplexing",
		}
	}

	// Mix verbCounter with session ID hash for variety across sessions
	seed := m.verbCounter
	if m.session != nil && len(m.session.ID) > 0 {
		for _, b := range m.session.ID {
			seed += int(b)
		}
	}
	return pool[seed%len(pool)]
}

// pulseText renders text with a pulsating brightness effect.
// Cycles through 4 brightness levels on the glider tick.
// scanText renders text with a glowing highlight that sweeps left-to-right and back.
// Each character gets a brightness based on its distance from the scan position.
func (m Model) scanText(text string) string {
	chars := []rune(text + "...")
	n := len(chars)
	if n == 0 {
		return ""
	}

	// Scan position bounces: 0→n→0→n... (ping-pong)
	// Each spinner tick moves the highlight by ~1 character
	cycle := 2 * n
	pos := m.spinnerTick % cycle
	if pos >= n {
		pos = cycle - pos // bounce back
	}

	// Brightness levels: bright at scan pos, dimmer further away
	var result strings.Builder
	for i, ch := range chars {
		dist := pos - i
		if dist < 0 {
			dist = -dist
		}

		var color string
		switch {
		case dist == 0:
			color = "255" // bright white — the scan point
		case dist == 1:
			color = "250"
		case dist == 2:
			color = "246"
		case dist <= 4:
			color = "243"
		default:
			color = "240" // base dim
		}

		result.WriteString(
			lipgloss.NewStyle().
				Foreground(lipgloss.Color(color)).
				Bold(dist <= 1).
				Render(string(ch)),
		)
	}
	return result.String()
}
