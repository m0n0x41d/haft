package cli

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// runEventMsg wraps a RunEvent for delivery into the BubbleTea update loop.
type runEventMsg RunEvent

// tickElapsedMsg fires every second to update the elapsed-time display.
type tickElapsedMsg time.Time

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------

var (
	sHeaderBar = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	sFooterBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	sSidebarTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1)

	sTaskPending = lipgloss.NewStyle().Faint(true)
	sTaskRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("14")) // cyan
	sTaskDone    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	sTaskFailed  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // red
	sTaskSkipped = lipgloss.NewStyle().Faint(true)

	sThinking  = lipgloss.NewStyle().Faint(true).Italic(true)
	sToolCall  = lipgloss.NewStyle().Foreground(lipgloss.Color("13")) // magenta
	sSummOK    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	sSummWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	sSummFail  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	sPhase     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	sBuildOK   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	sBuildFail = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	sInvOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	sInvFail   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// ---------------------------------------------------------------------------
// Task state tracked by the TUI
// ---------------------------------------------------------------------------

type tuiTask struct {
	id     string
	title  string
	status TaskStatus
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

// tuiModel is the BubbleTea model for the haft run TUI.
// Zero business logic — pure view layer.
type tuiModel struct {
	events <-chan RunEvent

	// layout
	width  int
	height int

	// sub-models
	viewport viewport.Model
	spinner  spinner.Model

	// header state
	decisionID string
	agentName  string
	mode       string // "auto" | "checkpointed"

	// task sidebar
	tasks       []tuiTask
	currentTask string

	// main panel content buffer (appended to, set into viewport)
	lines []string

	// timing
	startTime time.Time
	elapsed   time.Duration

	// pipeline finished
	done bool
}

// newTUIModel creates a tuiModel wired to an event channel.
func newTUIModel(events <-chan RunEvent) tuiModel {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

	return tuiModel{
		events:    events,
		viewport:  viewport.New(),
		spinner:   sp,
		startTime: time.Now(),
	}
}

// ---------------------------------------------------------------------------
// tea.Model interface
// ---------------------------------------------------------------------------

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.waitForEvent(),
		tickElapsed(),
	)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()

	case tea.KeyPressMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tickElapsedMsg:
		m.elapsed = time.Since(m.startTime)
		cmds = append(cmds, tickElapsed())

	case runEventMsg:
		m.handleEvent(RunEvent(msg))
		if !m.done {
			cmds = append(cmds, m.waitForEvent())
		}
	}

	// forward to viewport for scroll handling
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m tuiModel) View() tea.View {
	if m.width < 60 || m.height < 10 {
		v := tea.NewView("Terminal too small (need >= 60x10)")
		v.AltScreen = true
		return v
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	bodyHeight := m.height - 2 // header + footer = 1 line each

	sidebarWidth := m.width * 30 / 100
	mainWidth := m.width - sidebarWidth

	// main viewport
	m.viewport.SetWidth(mainWidth)
	m.viewport.SetHeight(bodyHeight)
	m.viewport.SoftWrap = true
	mainPanel := m.viewport.View()

	// sidebar
	sidebar := m.renderSidebar(sidebarWidth, bodyHeight)

	body := lipgloss.JoinHorizontal(lipgloss.Top, mainPanel, sidebar)

	screen := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	v := tea.NewView(screen)
	v.AltScreen = true
	return v
}

// ---------------------------------------------------------------------------
// Event handling — updates model state from RunEvents
// ---------------------------------------------------------------------------

func (m *tuiModel) handleEvent(e RunEvent) {
	switch {
	case e.PhaseBegan != nil:
		m.appendLine(sPhase.Render("--- " + e.PhaseBegan.Name + " ---"))

	case e.MetaInfo != nil:
		label := e.MetaInfo.Label
		value := e.MetaInfo.Value
		switch label {
		case "Decision":
			m.decisionID = value
		case "Agent":
			m.agentName = value
		case "Mode":
			m.mode = value
		}
		m.appendLine(fmt.Sprintf("  %s: %s", lipgloss.NewStyle().Faint(true).Render(label), value))

	case e.PlanLoaded != nil:
		m.tasks = make([]tuiTask, len(e.PlanLoaded.Tasks))
		for i, t := range e.PlanLoaded.Tasks {
			m.tasks[i] = tuiTask{id: t.ID, title: t.Title, status: TaskPending}
		}
		m.appendLine(fmt.Sprintf("  Plan: %d tasks", e.PlanLoaded.TaskCount))

	case e.TaskStatusChanged != nil:
		t := e.TaskStatusChanged
		m.updateTask(t.TaskID, t.TaskTitle, t.Status)
		switch t.Status {
		case TaskRunning:
			m.currentTask = t.TaskTitle
		case TaskPassed:
			m.appendLine(sTaskDone.Render(fmt.Sprintf("  ✓ %s (%ds)", t.TaskID, int(t.Elapsed.Seconds()))))
		case TaskFailed:
			msg := t.TaskID
			if t.Detail != "" {
				msg += ": " + t.Detail
			}
			m.appendLine(sTaskFailed.Render("  ✗ " + msg))
		case TaskSkipped:
			m.appendLine(sTaskSkipped.Render("  - " + t.TaskID + " skipped"))
		}

	case e.AgentChunk != nil:
		chunk := e.AgentChunk
		if chunk.Done {
			return
		}
		switch chunk.Kind {
		case ChunkThinking:
			m.appendLine(sThinking.Render("  " + truncateLine(chunk.Text, m.mainWidth()-4)))
		case ChunkToolUse:
			line := fmt.Sprintf("  ⚙ %s", chunk.ToolName)
			if chunk.ToolArgs != "" {
				remaining := m.mainWidth() - len(line) - 3
				if remaining > 10 {
					line += " " + truncateLine(chunk.ToolArgs, remaining)
				}
			}
			m.appendLine(sToolCall.Render(line))
		case ChunkText:
			for _, l := range strings.Split(chunk.Text, "\n") {
				m.appendLine("  " + l)
			}
		case ChunkRaw:
			m.appendLine("  " + chunk.Text)
		}

	case e.BuildResult != nil:
		if e.BuildResult.OK {
			m.appendLine(sBuildOK.Render(fmt.Sprintf("  ✓ %s passed", e.BuildResult.Command)))
		} else {
			m.appendLine(sBuildFail.Render(fmt.Sprintf("  ✗ %s failed", e.BuildResult.Command)))
			if e.BuildResult.Output != "" {
				for _, l := range strings.Split(e.BuildResult.Output, "\n") {
					m.appendLine("    " + l)
				}
			}
		}

	case e.InvariantResult != nil:
		inv := e.InvariantResult
		if inv.Pass {
			m.appendLine(sInvOK.Render(fmt.Sprintf("  ✓ [%s] %s", inv.Source, inv.Text)))
		} else {
			m.appendLine(sInvFail.Render(fmt.Sprintf("  ✗ [%s] %s", inv.Source, inv.Text)))
			if inv.Reason != "" {
				m.appendLine(sInvFail.Render("       " + inv.Reason))
			}
		}

	case e.Summary != nil:
		switch e.Summary.Level {
		case StatusOK:
			m.appendLine(sSummOK.Render("  ✓ " + e.Summary.Message))
		case StatusWarn:
			m.appendLine(sSummWarn.Render("  ⚠ " + e.Summary.Message))
		case StatusFail:
			m.appendLine(sSummFail.Render("  ✗ " + e.Summary.Message))
		}

	case e.PipelineDone != nil:
		m.elapsed = e.PipelineDone.Elapsed
		m.done = true
		if e.PipelineDone.Success {
			m.appendLine(sSummOK.Render("\n  Pipeline completed successfully"))
		} else {
			m.appendLine(sSummFail.Render("\n  Pipeline finished with failures"))
		}
		m.appendLine(fmt.Sprintf("  Duration: %ds", int(m.elapsed.Seconds())))
		m.appendLine("\n  Press q to exit.")
	}
}

// ---------------------------------------------------------------------------
// Layout helpers
// ---------------------------------------------------------------------------

func (m *tuiModel) recalcLayout() {
	mainWidth := m.width * 70 / 100
	bodyHeight := m.height - 2
	m.viewport.SetWidth(mainWidth)
	m.viewport.SetHeight(bodyHeight)
}

func (m tuiModel) mainWidth() int {
	w := m.width * 70 / 100
	if w < 20 {
		return 20
	}
	return w
}

func (m *tuiModel) appendLine(s string) {
	m.lines = append(m.lines, s)
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
	m.viewport.GotoBottom()
}

func (m *tuiModel) updateTask(id, title string, status TaskStatus) {
	for i := range m.tasks {
		if m.tasks[i].id == id {
			m.tasks[i].status = status
			return
		}
	}
	// Task not found — append (can happen if plan wasn't loaded yet)
	m.tasks = append(m.tasks, tuiTask{id: id, title: title, status: status})
}

// ---------------------------------------------------------------------------
// Render: header, footer, sidebar
// ---------------------------------------------------------------------------

func (m tuiModel) renderHeader() string {
	// Left: decision ID + agent
	left := m.decisionID
	if m.agentName != "" {
		left += " | " + m.agentName
	}
	if m.mode != "" {
		left += " | " + m.mode
	}

	// Right: task progress
	done, total := m.taskProgress()
	right := fmt.Sprintf("%d/%d", done, total)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2 // padding
	if gap < 1 {
		gap = 1
	}

	bar := left + strings.Repeat(" ", gap) + right
	return sHeaderBar.Width(m.width).Render(bar)
}

func (m tuiModel) renderFooter() string {
	elapsed := fmt.Sprintf("%ds", int(m.elapsed.Seconds()))

	task := m.currentTask
	if m.done {
		task = "done"
	}
	if len(task) > 40 {
		task = task[:37] + "..."
	}

	keys := "q:quit"

	left := elapsed
	if task != "" {
		left += " | " + task
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(keys) - 2
	if gap < 1 {
		gap = 1
	}

	bar := left + strings.Repeat(" ", gap) + keys
	return sFooterBar.Width(m.width).Render(bar)
}

func (m tuiModel) renderSidebar(width, height int) string {
	var b strings.Builder

	title := sSidebarTitle.Render("Tasks")
	b.WriteString(title)
	b.WriteRune('\n')

	maxTitleLen := width - 6 // icon + space + id + space + padding
	if maxTitleLen < 5 {
		maxTitleLen = 5
	}

	for _, t := range m.tasks {
		taskTitle := t.title
		if len(taskTitle) > maxTitleLen {
			taskTitle = taskTitle[:maxTitleLen-3] + "..."
		}

		var line string
		switch t.status {
		case TaskPending:
			line = sTaskPending.Render(fmt.Sprintf(" · %s %s", t.id, taskTitle))
		case TaskRunning:
			line = sTaskRunning.Render(fmt.Sprintf(" %s %s %s", m.spinner.View(), t.id, taskTitle))
		case TaskPassed:
			line = sTaskDone.Render(fmt.Sprintf(" ✓ %s %s", t.id, taskTitle))
		case TaskFailed:
			line = sTaskFailed.Render(fmt.Sprintf(" ✗ %s %s", t.id, taskTitle))
		case TaskSkipped:
			line = sTaskSkipped.Render(fmt.Sprintf(" - %s %s", t.id, taskTitle))
		}
		b.WriteString(line)
		b.WriteRune('\n')
	}

	content := b.String()

	// Pad to fill height
	contentLines := strings.Count(content, "\n") + 1
	for contentLines < height {
		content += "\n"
		contentLines++
	}

	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	return style.Render(content)
}

func (m tuiModel) taskProgress() (done, total int) {
	total = len(m.tasks)
	for _, t := range m.tasks {
		switch t.status {
		case TaskPassed, TaskFailed, TaskSkipped:
			done++
		}
	}
	return done, total
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

// waitForEvent returns a tea.Cmd that blocks until the next RunEvent arrives
// on the channel (or the channel closes).
func (m tuiModel) waitForEvent() tea.Cmd {
	ch := m.events
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return runEventMsg(RunEvent{PipelineDone: &PipelineDone{Success: true}})
		}
		return runEventMsg(e)
	}
}

func tickElapsed() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickElapsedMsg(t)
	})
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

func truncateLine(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
