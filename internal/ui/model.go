package ui

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/m0n0x41d/haft/internal/artifact"
)

// Tab identifies a dashboard view.
type Tab int

const (
	TabOverview Tab = iota
	TabProblems
	TabDecisions
	TabModules
)

var tabNames = []string{"Overview", "Problems", "Decisions", "Modules"}

const refreshInterval = 3 * time.Second

// HelpItem is a key binding shown in the help bar.
type HelpItem struct {
	Key  string
	Desc string
}

// View renders a single tab's content.
type View interface {
	Render(width, height int, styles Styles) string
	HandleKey(msg tea.KeyMsg) bool
	Title() string
	HelpKeys() []HelpItem
	UpdateData(data *BoardData)
}

// refreshMsg triggers a data reload from DB.
type refreshMsg struct{}

// Model is the top-level bubbletea model for the dashboard.
type Model struct {
	data   *BoardData
	styles Styles
	views  map[Tab]View
	active Tab
	width  int
	height int
	ready  bool

	// DB connection for live refresh
	store       *artifact.Store
	db          *sql.DB
	projectName string
	projectRoot string
}

// New creates a new dashboard model with live refresh capability.
func New(data *BoardData, store *artifact.Store, db *sql.DB, projectName, projectRoot string) Model {
	m := Model{
		data:        data,
		active:      TabOverview,
		views:       make(map[Tab]View),
		store:       store,
		db:          db,
		projectName: projectName,
		projectRoot: projectRoot,
	}
	m.views[TabOverview] = NewOverviewView(data)
	m.views[TabProblems] = NewProblemsView(data)
	m.views[TabDecisions] = NewDecisionsView(data)
	m.views[TabModules] = NewModulesView(data)
	return m
}

// CriticalCount returns the number of critical health issues.
func (m Model) CriticalCount() int {
	return m.data.CriticalCount
}

func (m Model) Init() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		return refreshMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.styles = NewStyles(m.width)
		m.ready = true
		return m, nil

	case refreshMsg:
		m.reload()
		// Schedule next tick
		return m, tea.Tick(refreshInterval, func(time.Time) tea.Msg {
			return refreshMsg{}
		})

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.active = (m.active + 1) % Tab(len(tabNames))
			return m, nil
		case "shift+tab":
			m.active = (m.active - 1 + Tab(len(tabNames))) % Tab(len(tabNames))
			return m, nil
		case "1":
			m.active = TabOverview
			return m, nil
		case "2":
			m.active = TabProblems
			return m, nil
		case "3":
			m.active = TabDecisions
			return m, nil
		case "4":
			m.active = TabModules
			return m, nil
		}

		if view, ok := m.views[m.active]; ok {
			view.HandleKey(msg)
		}
	}

	return m, nil
}

// reload refreshes data from DB without resetting view state (cursor, detail mode).
func (m *Model) reload() {
	data, err := LoadBoardData(m.store, m.db, m.projectName, m.projectRoot)
	if err != nil {
		return
	}
	m.data = data
	for _, v := range m.views {
		v.UpdateData(data)
	}
}

func (m Model) View() tea.View {
	if !m.ready {
		return tea.NewView("Loading...")
	}

	header := m.renderHeader()
	tabBar := m.renderTabBar()
	helpBar := m.renderHelp()

	headerH := lipgloss.Height(header)
	tabBarH := lipgloss.Height(tabBar)
	helpH := lipgloss.Height(helpBar)
	contentH := m.height - headerH - tabBarH - helpH

	content := ""
	if view, ok := m.views[m.active]; ok {
		content = view.Render(m.width-2, contentH, m.styles)
	}

	// Pad content to push help to bottom
	contentLines := lipgloss.Height(content)
	if contentLines < contentH {
		content += strings.Repeat("\n", contentH-contentLines)
	}

	result := header + "\n" + tabBar + "\n" + content + helpBar

	v := tea.NewView(result)
	v.AltScreen = true
	return v
}

func (m Model) renderHeader() string {
	t := m.styles.Theme

	projectStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Cyan)
	statsStyle := lipgloss.NewStyle().Foreground(t.Dim)

	decCount := len(m.data.Decisions)
	modCount := 0
	covPct := 0
	if m.data.CoverageReport != nil {
		modCount = m.data.CoverageReport.TotalModules
		if modCount > 0 {
			covPct = (m.data.CoverageReport.CoveredCount + m.data.CoverageReport.PartialCount) * 100 / modCount
		}
	}

	health := m.styles.OK.Render("healthy")
	if m.data.CriticalCount > 0 {
		health = m.styles.Error.Render(fmt.Sprintf("%d critical", m.data.CriticalCount))
	}

	header := fmt.Sprintf(" %s  %s  %s",
		projectStyle.Render(m.data.ProjectName),
		statsStyle.Render(fmt.Sprintf("%d decisions · %d modules · %d%% governed",
			decCount, modCount, covPct)),
		health,
	)

	return m.styles.Header.Render(header)
}

func (m Model) renderTabBar() string {
	var tabs []string

	for i, name := range tabNames {
		title := name
		if view, ok := m.views[Tab(i)]; ok {
			title = view.Title()
		}
		label := fmt.Sprintf(" %d %s ", i+1, title)

		isActive := Tab(i) == m.active
		isFirst := i == 0
		isLast := i == len(tabNames)-1

		border := lipgloss.RoundedBorder()
		if isActive {
			border.Bottom = " "
			border.BottomLeft = "│"
			border.BottomRight = "│"
		} else {
			border.Bottom = "─"
			border.BottomLeft = "┴"
			border.BottomRight = "┴"
		}
		if isFirst && isActive {
			border.BottomLeft = "│"
		} else if isFirst {
			border.BottomLeft = "└"
		}
		if isLast && isActive {
			border.BottomRight = "│"
		} else if isLast {
			border.BottomRight = "┘"
		}

		style := lipgloss.NewStyle().
			Border(border).
			BorderForeground(m.styles.Theme.Border).
			Padding(0, 1)

		if isActive {
			style = style.Bold(true).Foreground(m.styles.Theme.Bold)
		} else {
			style = style.Foreground(m.styles.Theme.Dim)
		}

		tabs = append(tabs, style.Render(label))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	// Fill remaining width with bottom border
	rowWidth := lipgloss.Width(row)
	if rowWidth < m.width {
		gap := lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(m.styles.Theme.Border).
			Width(m.width - rowWidth).
			Render("")
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap)
	}

	return row
}

func (m Model) renderHelp() string {
	s := m.styles

	items := []HelpItem{
		{"tab", "switch"},
		{"1-4", "jump"},
	}

	if view, ok := m.views[m.active]; ok {
		items = append(items, view.HelpKeys()...)
	}

	items = append(items, HelpItem{"q", "quit"})

	var parts []string
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%s %s", s.HelpKey.Render(item.Key), item.Desc))
	}
	return s.HelpBar.Render(" " + strings.Join(parts, " · "))
}
