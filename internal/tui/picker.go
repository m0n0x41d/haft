package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/session"
)

// sessionItem implements list.DefaultItem for the session picker.
type sessionItem struct {
	session     agent.Session
	lastMessage string
}

func (i sessionItem) Title() string {
	age := time.Since(i.session.UpdatedAt)
	var ageStr string
	switch {
	case age < time.Minute:
		ageStr = "just now"
	case age < time.Hour:
		ageStr = fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		ageStr = fmt.Sprintf("%dh ago", int(age.Hours()))
	default:
		ageStr = i.session.UpdatedAt.Format("Jan 2")
	}

	sid := i.session.ID
	if len(sid) > 12 {
		sid = sid[:12]
	}

	return fmt.Sprintf("%s  %s  %s", sid, i.session.Model, ageStr)
}

func (i sessionItem) Description() string {
	msg := i.lastMessage
	if msg == "" {
		msg = "(empty session)"
	}
	// Single line, truncated
	msg = strings.ReplaceAll(msg, "\n", " ")
	if r := []rune(msg); len(r) > 80 {
		msg = string(r[:77]) + "..."
	}
	return msg
}

func (i sessionItem) FilterValue() string {
	return i.session.ID + " " + i.lastMessage
}

// SessionPickerResult is sent when the user selects a session from the picker.
type SessionPickerResult struct {
	Session agent.Session
}

// SessionPickerCancel is sent when the user cancels the picker.
type SessionPickerCancel struct{}

// buildSessionPicker creates a list.Model for session selection.
func buildSessionPicker(
	ctx context.Context,
	store session.SessionStore,
	msgStore session.MessageStore,
	width, height int,
) (list.Model, error) {
	sessions, err := store.ListRecent(ctx, 20)
	if err != nil {
		return list.Model{}, fmt.Errorf("list sessions: %w", err)
	}

	items := make([]list.Item, 0, len(sessions))
	for _, s := range sessions {
		lastMsg, _ := msgStore.LastUserMessage(ctx, s.ID)
		// Skip empty sessions (no messages)
		if lastMsg == "" {
			continue
		}
		items = append(items, sessionItem{
			session:     s,
			lastMessage: lastMsg,
		})
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(lipgloss.Color("39")).
		PaddingLeft(1)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(lipgloss.Color("39")).
		PaddingLeft(1)

	l := list.New(items, delegate, width-4, height-4)
	l.Title = "Resume Session"
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		PaddingLeft(1)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.SetShowStatusBar(false)

	return l, nil
}

// handleSessionPickerKey processes keys in the session picker state.
func (m Model) handleSessionPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()

	// Escape or Ctrl+C cancels
	if key.Code == tea.KeyEscape || (key.Mod == tea.ModCtrl && key.Code == 'c') {
		m.state = stateInput
		return m, m.input.Focus()
	}

	// Enter selects
	if key.Code == tea.KeyEnter {
		selected := m.picker.SelectedItem()
		if selected == nil {
			m.state = stateInput
			return m, m.input.Focus()
		}
		si, ok := selected.(sessionItem)
		if !ok {
			m.state = stateInput
			return m, m.input.Focus()
		}

		// Load the session's messages and switch to it
		m.session = &si.session
		msgs, err := m.sessionMsgStore.ListBySession(context.Background(), si.session.ID)
		if err == nil {
			m.messages = restoreViewMessages(msgs)
		}

		// Restore active cycle state
		if m.cycleStore != nil {
			if cycle, err := m.cycleStore.GetActiveCycle(context.Background(), si.session.ID); err == nil && cycle != nil {
				m.cycleID = cycle.ID
				m.problemRef = cycle.ProblemRef
				m.portfolioRef = cycle.PortfolioRef
				m.decisionRef = cycle.DecisionRef
				m.cycleStatus = cycle.Status
				// Phase is derived from cycle state in renderCycleInfo
			}
		}

		m.state = stateInput
		m.refreshChat()
		return m, m.input.Focus()
	}

	// Forward to list
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	return m, cmd
}

// restoreViewMessages converts persisted agent.Messages back into viewMessages,
// reconstructing tool call structure (name, args, output) from Parts.
func restoreViewMessages(msgs []agent.Message) []viewMessage {
	var result []viewMessage

	// Index tool results by ToolCallID for fast lookup
	toolResults := make(map[string]agent.ToolResultPart)
	for _, msg := range msgs {
		if msg.Role != agent.RoleTool {
			continue
		}
		for _, p := range msg.Parts {
			if tr, ok := p.(agent.ToolResultPart); ok {
				toolResults[tr.ToolCallID] = tr
			}
		}
	}

	for _, msg := range msgs {
		switch msg.Role {
		case agent.RoleUser:
			text := msg.Text()
			if text == "" {
				continue
			}
			result = append(result, viewMessage{
				ID:   msg.ID,
				Role: agent.RoleUser,
				Text: text,
			})

		case agent.RoleAssistant:
			vm := viewMessage{
				ID:   msg.ID,
				Role: agent.RoleAssistant,
				Text: msg.Text(),
			}

			// Restore tool calls from ToolCallParts
			for _, p := range msg.Parts {
				tc, ok := p.(agent.ToolCallPart)
				if !ok {
					continue
				}
				vt := viewTool{
					CallID:  tc.ToolCallID,
					Name:    tc.ToolName,
					Args:    tc.Arguments,
					Running: false,
				}
				// Match with tool result
				if tr, found := toolResults[tc.ToolCallID]; found {
					vt.Output = tr.Content
					vt.IsError = tr.IsError
				}
				vm.Tools = append(vm.Tools, vt)
			}

			result = append(result, vm)

			// RoleTool messages are consumed via toolResults map above
		}
	}

	return result
}
